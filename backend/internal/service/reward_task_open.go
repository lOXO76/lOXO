package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/rewardtaskevent"
	"github.com/Wei-Shaw/sub2api/ent/rewardtaskpartner"
	"github.com/Wei-Shaw/sub2api/ent/rewardtaskrecord"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const RewardOpenMaxBodyBytes = 64 * 1024

var (
	rewardOpenNoncePattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)
	rewardOpenAmountPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,2})?$`)
)

type RewardOpenSignature struct {
	AppKey    string
	Timestamp string
	Nonce     string
	Signature string
}

func validateRewardOpenSignatureHeaders(headers RewardOpenSignature) error {
	if headers.AppKey == "" || len(headers.AppKey) > 64 || !rewardOpenNoncePattern.MatchString(headers.Nonce) ||
		len(headers.Signature) != 64 || headers.Signature != strings.ToLower(headers.Signature) {
		return rewardTaskError("REWARD_SIGNATURE_MISSING")
	}
	if _, err := hex.DecodeString(headers.Signature); err != nil {
		return rewardTaskError("REWARD_SIGNATURE_MISSING")
	}
	timestamp, err := strconv.ParseInt(headers.Timestamp, 10, 64)
	if err != nil || timestamp <= 0 || strconv.FormatInt(timestamp, 10) != headers.Timestamp {
		return rewardTaskError("REWARD_SIGNATURE_MISSING")
	}
	return nil
}

func verifyRewardOpenSignature(headers RewardOpenSignature, secret string, body []byte, now time.Time) error {
	if err := validateRewardOpenSignatureHeaders(headers); err != nil {
		return err
	}
	timestamp, _ := strconv.ParseInt(headers.Timestamp, 10, 64)
	if timestamp < now.Unix()-300 || timestamp > now.Unix()+300 {
		return rewardTaskError("REWARD_TIMESTAMP_EXPIRED")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(headers.Timestamp + "\n" + headers.Nonce + "\n"))
	_, _ = mac.Write(body)
	signature, _ := hex.DecodeString(headers.Signature)
	if !hmac.Equal(mac.Sum(nil), signature) {
		return rewardTaskError("REWARD_SIGNATURE_INVALID")
	}
	return nil
}

func (s *RewardTaskService) consumeRewardOpenNonce(ctx context.Context, appKey, nonce string) error {
	if s.nonceStore == nil {
		slog.Warn("reward-open nonce Redis unavailable; allowing request")
		return nil
	}
	ok, err := s.nonceStore.ConsumeNonce(ctx, "reward_open:nonce:"+appKey+":"+nonce, 600*time.Second)
	if err != nil {
		slog.Warn("reward-open nonce Redis failed; allowing request", "error", err)
		return nil
	}
	if !ok {
		return rewardTaskError("REWARD_NONCE_REPLAYED")
	}
	return nil
}

// AuthenticateRewardOpen 不消费无效签名的 nonce，避免攻击者提前占用合法请求的 nonce。
func (s *RewardTaskService) AuthenticateRewardOpen(ctx context.Context, headers RewardOpenSignature, body []byte) (*dbent.RewardTaskPartner, error) {
	if err := validateRewardOpenSignatureHeaders(headers); err != nil {
		return nil, err
	}
	partner, err := s.entClient.RewardTaskPartner.Query().Where(rewardtaskpartner.AppKeyEQ(headers.AppKey)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_PARTNER_INVALID")
	}
	if err != nil {
		return nil, err
	}
	if !partner.Enabled {
		return partner, rewardTaskError("REWARD_PARTNER_INVALID")
	}
	if s.encryptor == nil {
		return partner, errors.New("reward-open secret encryptor is not configured")
	}
	secret, err := s.encryptor.Decrypt(partner.AppSecretEncrypted)
	if err != nil {
		return partner, err
	}
	now := time.Now()
	if err := verifyRewardOpenSignature(headers, secret, body, now); err != nil {
		return partner, err
	}
	if err := s.consumeRewardOpenNonce(ctx, headers.AppKey, headers.Nonce); err != nil {
		return partner, err
	}
	if err := s.entClient.RewardTaskPartner.UpdateOneID(partner.ID).SetLastEventAt(now).Exec(ctx); err != nil {
		return partner, err
	}
	return partner, nil
}

func (s *RewardTaskService) rewardOpenEnabled(ctx context.Context) error {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return infraerrors.Forbidden("REWARD_TASKS_DISABLED", "reward tasks are disabled")
	}
	return nil
}

type RewardOpenPingResult struct {
	PartnerName string `json:"partner_name"`
	AppKey      string `json:"app_key"`
	ServerTime  int64  `json:"server_time"`
}

func (s *RewardTaskService) OpenPing(ctx context.Context, partner *dbent.RewardTaskPartner) (*RewardOpenPingResult, error) {
	if err := s.rewardOpenEnabled(ctx); err != nil {
		return nil, err
	}
	return &RewardOpenPingResult{PartnerName: partner.Name, AppKey: partner.AppKey, ServerTime: time.Now().Unix()}, nil
}

type RewardOpenTicketView struct {
	Ticket            string    `json:"ticket"`
	TaskID            int64     `json:"task_id"`
	TaskTitle         string    `json:"task_title"`
	TaskType          string    `json:"task_type"`
	Period            string    `json:"period"`
	PeriodKey         string    `json:"period_key"`
	Status            string    `json:"status"`
	ExpiresAt         time.Time `json:"expires_at"`
	Expired           bool      `json:"expired"`
	ExternalUserID    string    `json:"external_user_id"`
	BrowseSeconds     int       `json:"browse_seconds"`
	ProgressSeconds   int       `json:"progress_seconds"`
	MinRechargeAmount float64   `json:"min_recharge_amount"`
	RechargeTotal     float64   `json:"recharge_total"`
}

func (s *RewardTaskService) QueryRewardOpenTicket(ctx context.Context, partnerID int64, ticket string) (*RewardOpenTicketView, error) {
	if err := s.rewardOpenEnabled(ctx); err != nil {
		return nil, err
	}
	record, err := s.entClient.RewardTaskRecord.Query().Where(rewardtaskrecord.TicketEQ(ticket)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_TICKET_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	task, err := s.entClient.RewardTask.Get(ctx, record.TaskID)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_TASK_UNAVAILABLE")
	}
	if err != nil {
		return nil, err
	}
	if task.PartnerID == nil || *task.PartnerID != partnerID {
		return nil, rewardTaskError("REWARD_TICKET_PARTNER_MISMATCH")
	}
	return &RewardOpenTicketView{
		Ticket: record.Ticket, TaskID: task.ID, TaskTitle: task.Title, TaskType: task.TaskType,
		Period: task.Period, PeriodKey: record.PeriodKey, Status: record.Status,
		ExpiresAt: record.TicketExpiresAt, Expired: !record.TicketExpiresAt.After(time.Now()),
		ExternalUserID: record.ExternalUserID, BrowseSeconds: task.BrowseSeconds,
		ProgressSeconds: rewardProgressSeconds(record.ProgressMs, task.BrowseSeconds, record.Status == "completed"), MinRechargeAmount: task.MinRechargeAmount, RechargeTotal: record.RechargeTotal,
	}, nil
}

type rewardOpenEventInput struct {
	EventID        string          `json:"event_id"`
	EventType      string          `json:"event_type"`
	Ticket         string          `json:"ticket"`
	ExternalUserID string          `json:"external_user_id"`
	Amount         json.RawMessage `json:"amount"`
	OrderID        string          `json:"order_id"`
	amountText     string
}

func parseRewardOpenEvent(body []byte) (*rewardOpenEventInput, error) {
	var input rewardOpenEventInput
	if len(body) > RewardOpenMaxBodyBytes || json.Unmarshal(body, &input) != nil ||
		strings.TrimSpace(input.EventID) == "" || utf8.RuneCountInString(input.EventID) > 128 ||
		strings.TrimSpace(input.ExternalUserID) == "" || utf8.RuneCountInString(input.ExternalUserID) > 128 ||
		input.Ticket == "" || len(input.Ticket) > 64 || utf8.RuneCountInString(input.OrderID) > 128 ||
		(input.EventType != "register" && input.EventType != "recharge") {
		return nil, rewardTaskError("REWARD_EVENT_INVALID")
	}
	if input.EventType == "recharge" {
		text := string(input.Amount)
		if len(text) > 0 && text[0] == '"' {
			if err := json.Unmarshal(input.Amount, &text); err != nil {
				return nil, rewardTaskError("REWARD_EVENT_INVALID")
			}
		}
		if !rewardOpenAmountPattern.MatchString(text) {
			return nil, rewardTaskError("REWARD_EVENT_INVALID")
		}
		amount, err := strconv.ParseFloat(text, 64)
		integerPart := strings.SplitN(text, ".", 2)[0]
		// 用十进制原文写 NUMERIC，不能让 float64 舍入丢失充值分位。
		if err != nil || amount <= 0 || len(strings.TrimLeft(integerPart, "0")) > 18 || math.IsInf(amount, 0) {
			return nil, rewardTaskError("REWARD_EVENT_INVALID")
		}
		input.amountText = text
	}
	return &input, nil
}

type RewardOpenProgress struct {
	RechargeTotal     float64 `json:"recharge_total"`
	MinRechargeAmount float64 `json:"min_recharge_amount"`
	ProgressSeconds   int     `json:"progress_seconds"`
	BrowseSeconds     int     `json:"browse_seconds"`
}

type RewardOpenEventResult struct {
	EventID   string             `json:"event_id"`
	Duplicate bool               `json:"duplicate"`
	Result    string             `json:"result"`
	TaskID    int64              `json:"task_id"`
	TaskType  string             `json:"task_type"`
	Status    string             `json:"status"`
	Rewarded  bool               `json:"rewarded"`
	Progress  RewardOpenProgress `json:"progress"`
}

func rewardOpenEventResult(eventID, result string, duplicate bool, record *dbent.RewardTaskRecord, task *dbent.RewardTask) *RewardOpenEventResult {
	return &RewardOpenEventResult{
		EventID: eventID, Result: result, Duplicate: duplicate, TaskID: task.ID, TaskType: task.TaskType,
		Status: record.Status, Rewarded: result == "completed",
		Progress: RewardOpenProgress{RechargeTotal: record.RechargeTotal, MinRechargeAmount: task.MinRechargeAmount,
			ProgressSeconds: rewardProgressSeconds(record.ProgressMs, task.BrowseSeconds, record.Status == "completed"), BrowseSeconds: task.BrowseSeconds},
	}
}

func rewardOpenAuditPayload(body []byte) map[string]any {
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if len(body) <= RewardOpenMaxBodyBytes && json.Valid(body) && decoder.Decode(&payload) == nil && payload != nil {
		return payload
	}
	if len(body) > 4096 {
		body = body[:4096]
	}
	return map[string]any{"raw": strings.ToValidUTF8(string(body), "")}
}

func rewardOpenTruncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

func saveRewardOpenEvent(ctx context.Context, client *dbent.Client, partnerID int64, body []byte, remoteIP string, input *rewardOpenEventInput, record *dbent.RewardTaskRecord, result, errorCode string) error {
	payload := rewardOpenAuditPayload(body)
	if input == nil {
		// 类型不符/不合法 JSON 仍保留可解析的审计定位字段。
		input = &rewardOpenEventInput{}
		input.EventID, _ = payload["event_id"].(string)
		input.EventType, _ = payload["event_type"].(string)
		input.Ticket, _ = payload["ticket"].(string)
		input.ExternalUserID, _ = payload["external_user_id"].(string)
		input.OrderID, _ = payload["order_id"].(string)
	}
	var taskID, recordID, userID any
	if record != nil {
		taskID, recordID, userID = record.TaskID, record.ID, record.UserID
	}
	amount := input.amountText
	if amount == "" {
		amount = "0"
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = client.ExecContext(ctx, `INSERT INTO reward_task_events
		(partner_id,event_id,event_type,ticket,task_id,record_id,user_id,external_user_id,amount,order_id,result,error_code,remote_ip,payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::numeric,$10,$11,$12,$13,$14::jsonb)`,
		partnerID, rewardOpenTruncate(input.EventID, 128), rewardOpenTruncate(input.EventType, 20),
		rewardOpenTruncate(input.Ticket, 64), taskID, recordID, userID, rewardOpenTruncate(input.ExternalUserID, 128),
		amount, rewardOpenTruncate(input.OrderID, 128), result, errorCode, rewardOpenTruncate(remoteIP, 64), string(encoded))
	return err
}

func (s *RewardTaskService) LogRejectedRewardOpenEvent(ctx context.Context, partnerID int64, body []byte, remoteIP string, rejection error) {
	if s == nil || s.entClient == nil {
		return
	}
	// 客户端断开时仍尽量留存审计，不把网络取消传播给日志写入。
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	var record *dbent.RewardTaskRecord
	input, _ := parseRewardOpenEvent(body)
	payload := rewardOpenAuditPayload(body)
	ticket, _ := payload["ticket"].(string)
	if ticket != "" && len(ticket) <= 64 {
		record, _ = s.entClient.RewardTaskRecord.Query().Where(rewardtaskrecord.TicketEQ(ticket)).Only(auditCtx)
	}
	err := saveRewardOpenEvent(auditCtx, s.entClient, partnerID, body, remoteIP, input, record, "rejected", infraerrors.Reason(rejection))
	if err != nil {
		slog.Error("reward-open rejected event audit failed", "partner_id", partnerID, "error", err)
	}
}

// 同一 event_id 先串行化再读日志，避免不同票据上的并发请求同时累加或发奖。
func lockRewardOpenEvent(ctx context.Context, tx *dbent.Tx, key string) error {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte("reward-open:" + key))
	var rows entsql.Rows
	if err := tx.Client().Driver().Query(ctx, "SELECT pg_advisory_xact_lock($1)", []any{int64(hash.Sum64())}, &rows); err != nil {
		return err
	}
	return rows.Close()
}

func (s *RewardTaskService) ProcessRewardOpenEvent(ctx context.Context, partnerID int64, body []byte, remoteIP string) (*RewardOpenEventResult, error) {
	if err := s.rewardOpenEnabled(ctx); err != nil {
		s.LogRejectedRewardOpenEvent(ctx, partnerID, body, remoteIP, err)
		return nil, err
	}
	input, err := parseRewardOpenEvent(body)
	if err != nil {
		s.LogRejectedRewardOpenEvent(ctx, partnerID, body, remoteIP, err)
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		s.LogRejectedRewardOpenEvent(ctx, partnerID, body, remoteIP, err)
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, rewardedUserID, err := s.processRewardOpenEventTx(ctx, tx, partnerID, body, remoteIP, input)
	if err != nil {
		_ = tx.Rollback()
		s.LogRejectedRewardOpenEvent(ctx, partnerID, body, remoteIP, err)
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		s.LogRejectedRewardOpenEvent(ctx, partnerID, body, remoteIP, err)
		return nil, err
	}
	if rewardedUserID != 0 {
		s.invalidateBalanceCaches(ctx, rewardedUserID)
	}
	// 业务拒绝也提交事务：保留 recharge_total、绑定信息和 rejected 审计。
	if result.rejection != nil {
		return nil, result.rejection
	}
	return result.response, nil
}

type rewardOpenEventTxResult struct {
	response  *RewardOpenEventResult
	rejection error
}

func (s *RewardTaskService) processRewardOpenEventTx(ctx context.Context, tx *dbent.Tx, partnerID int64, body []byte, remoteIP string, input *rewardOpenEventInput) (*rewardOpenEventTxResult, int64, error) {
	if err := lockRewardOpenEvent(ctx, tx, fmt.Sprintf("event:%d:%s", partnerID, input.EventID)); err != nil {
		return nil, 0, err
	}
	client := tx.Client()
	previous, err := client.RewardTaskEvent.Query().Where(rewardtaskevent.PartnerIDEQ(partnerID), rewardtaskevent.EventIDEQ(input.EventID), rewardtaskevent.ResultNEQ("rejected")).Only(ctx)
	if err == nil {
		if previous.RecordID == nil || previous.TaskID == nil {
			return nil, 0, errors.New("reward-open accepted event has no record")
		}
		record, err := client.RewardTaskRecord.Get(ctx, *previous.RecordID)
		if err != nil {
			return nil, 0, err
		}
		task, err := client.RewardTask.Get(mixins.SkipSoftDelete(ctx), *previous.TaskID)
		if err != nil {
			return nil, 0, err
		}
		return &rewardOpenEventTxResult{response: rewardOpenEventResult(previous.EventID, previous.Result, true, record, task)}, 0, nil
	}
	if !dbent.IsNotFound(err) {
		return nil, 0, err
	}
	retainedEvent, err := client.RewardTaskEvent.Query().Where(rewardtaskevent.PartnerIDEQ(partnerID), rewardtaskevent.EventIDEQ(input.EventID), rewardtaskevent.ResultEQ("rejected"), rewardtaskevent.ErrorCodeEQ("REWARD_TASK_QUOTA_EXHAUSTED")).Order(dbent.Desc(rewardtaskevent.FieldID)).First(ctx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, 0, err
	}
	// 已计入的充值不可搬到另一个票据/账号，否则两边将来都可能领奖。
	if retainedEvent != nil && (retainedEvent.Ticket != input.Ticket || retainedEvent.ExternalUserID != input.ExternalUserID) {
		return s.rejectRewardOpenEventTx(ctx, client, partnerID, body, remoteIP, input, nil,
			infraerrors.BadRequest("REWARD_EVENT_INVALID", "an event already counted toward recharge cannot change ticket or external_user_id"))
	}
	record, err := client.RewardTaskRecord.Query().Where(rewardtaskrecord.TicketEQ(input.Ticket)).ForUpdate().Only(ctx)
	if dbent.IsNotFound(err) {
		return s.rejectRewardOpenEventTx(ctx, client, partnerID, body, remoteIP, input, nil, rewardTaskError("REWARD_TICKET_NOT_FOUND"))
	}
	if err != nil {
		return nil, 0, err
	}
	task, err := client.RewardTask.Get(ctx, record.TaskID)
	if dbent.IsNotFound(err) {
		return s.rejectRewardOpenEventTx(ctx, client, partnerID, body, remoteIP, input, record, rewardTaskError("REWARD_TASK_UNAVAILABLE"))
	}
	if err != nil {
		return nil, 0, err
	}
	var rejection error
	txService := *s
	txService.entClient = client
	switch {
	case task.PartnerID == nil || *task.PartnerID != partnerID:
		rejection = rewardTaskError("REWARD_TICKET_PARTNER_MISMATCH")
	case task.VerifyMode != "callback" || input.EventType != task.TaskType:
		rejection = rewardTaskError("REWARD_EVENT_TYPE_MISMATCH")
	default:
		rejection = txService.taskAvailable(ctx, task, time.Now())
		if rejection == nil && !record.TicketExpiresAt.After(time.Now()) {
			rejection = rewardTaskError("REWARD_TICKET_EXPIRED")
		}
	}
	if rejection != nil {
		return s.rejectRewardOpenEventTx(ctx, client, partnerID, body, remoteIP, input, record, rejection)
	}
	if record.Status == "completed" {
		if err := saveRewardOpenEvent(ctx, client, partnerID, body, remoteIP, input, record, "ignored", ""); err != nil {
			return nil, 0, err
		}
		return &rewardOpenEventTxResult{response: rewardOpenEventResult(input.EventID, "ignored", false, record, task)}, 0, nil
	}
	if record.ExternalUserID != "" && record.ExternalUserID != input.ExternalUserID {
		return s.rejectRewardOpenEventTx(ctx, client, partnerID, body, remoteIP, input, record, rewardTaskError("REWARD_EXTERNAL_USER_MISMATCH"))
	}
	if record.ExternalUserID == "" {
		if err := lockRewardOpenEvent(ctx, tx, fmt.Sprintf("external:%d:%s:%s", task.ID, record.PeriodKey, input.ExternalUserID)); err != nil {
			return nil, 0, err
		}
		used, err := client.RewardTaskRecord.Query().Where(rewardtaskrecord.TaskIDEQ(task.ID), rewardtaskrecord.PeriodKeyEQ(record.PeriodKey), rewardtaskrecord.ExternalUserIDEQ(input.ExternalUserID), rewardtaskrecord.IDNEQ(record.ID)).Exist(ctx)
		if err != nil {
			return nil, 0, err
		}
		if used {
			return s.rejectRewardOpenEventTx(ctx, client, partnerID, body, remoteIP, input, record, rewardTaskError("REWARD_EXTERNAL_USER_USED"))
		}
		record, err = client.RewardTaskRecord.UpdateOneID(record.ID).SetExternalUserID(input.ExternalUserID).Save(ctx)
		if err != nil {
			if dbent.IsConstraintError(err) {
				return nil, 0, rewardTaskError("REWARD_EXTERNAL_USER_USED")
			}
			return nil, 0, err
		}
	}
	if input.EventType == "recharge" {
		// 名额不足时上次累加已经提交；同一事件重试只尝试发奖，不重复计入充值。
		retained, err := client.RewardTaskEvent.Query().Where(rewardtaskevent.PartnerIDEQ(partnerID), rewardtaskevent.EventIDEQ(input.EventID), rewardtaskevent.RecordIDEQ(record.ID), rewardtaskevent.TicketEQ(input.Ticket), rewardtaskevent.ResultEQ("rejected"), rewardtaskevent.ErrorCodeEQ("REWARD_TASK_QUOTA_EXHAUSTED")).Order(dbent.Desc(rewardtaskevent.FieldID)).First(ctx)
		if err != nil && !dbent.IsNotFound(err) {
			return nil, 0, err
		}
		previousAmount := "0"
		if retained != nil {
			rows, err := client.QueryContext(ctx, `SELECT amount::text FROM reward_task_events WHERE id=$1`, retained.ID)
			if err != nil {
				return nil, 0, err
			}
			if rows.Next() {
				err = rows.Scan(&previousAmount)
			}
			rowsErr := rows.Err()
			_ = rows.Close()
			if err != nil {
				return nil, 0, err
			}
			if rowsErr != nil {
				return nil, 0, rowsErr
			}
		}
		_, err = client.ExecContext(ctx, `UPDATE reward_task_records SET recharge_total=recharge_total+$2::numeric-$3::numeric,updated_at=$4 WHERE id=$1`, record.ID, input.amountText, previousAmount, time.Now())
		if err != nil {
			return nil, 0, err
		}
		record, err = client.RewardTaskRecord.Get(ctx, record.ID)
		if err != nil {
			return nil, 0, err
		}
	}
	result := "accepted"
	var rewardedUserID int64
	if input.EventType == "register" || record.RechargeTotal >= task.MinRechargeAmount {
		rewarded, err := s.completeRewardTask(ctx, tx, record, task, time.Now())
		if err != nil {
			if infraerrors.Reason(err) == "REWARD_TASK_QUOTA_EXHAUSTED" {
				return s.rejectRewardOpenEventTx(ctx, client, partnerID, body, remoteIP, input, record, err)
			}
			return nil, 0, err
		}
		if rewarded {
			result = "completed"
			rewardedUserID = record.UserID
		} else {
			result = "ignored"
		}
		record, err = client.RewardTaskRecord.Get(ctx, record.ID)
		if err != nil {
			return nil, 0, err
		}
	}
	if err := saveRewardOpenEvent(ctx, client, partnerID, body, remoteIP, input, record, result, ""); err != nil {
		return nil, 0, err
	}
	return &rewardOpenEventTxResult{response: rewardOpenEventResult(input.EventID, result, false, record, task)}, rewardedUserID, nil
}

func (s *RewardTaskService) rejectRewardOpenEventTx(ctx context.Context, client *dbent.Client, partnerID int64, body []byte, remoteIP string, input *rewardOpenEventInput, record *dbent.RewardTaskRecord, rejection error) (*rewardOpenEventTxResult, int64, error) {
	if err := saveRewardOpenEvent(ctx, client, partnerID, body, remoteIP, input, record, "rejected", infraerrors.Reason(rejection)); err != nil {
		return nil, 0, err
	}
	return &rewardOpenEventTxResult{rejection: rejection}, 0, nil
}

func rewardSDKOriginAllowed(domains []string, origin, referer string) bool {
	if len(domains) == 0 {
		return true
	}
	source := origin
	if source == "" {
		source = referer
	}
	parsed, err := url.Parse(source)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	for _, domain := range domains {
		domain = strings.ToLower(domain)
		if strings.HasPrefix(domain, "*.") {
			if strings.HasSuffix(host, domain[1:]) && len(host) > len(domain)-1 {
				return true
			}
		} else if host == domain {
			return true
		}
	}
	return false
}

// rewardProgressSeconds 把毫秒进度换算成对外展示的秒数（所有接口统一走这里）。
// SDK 按固定 1 秒节拍心跳，累计值总落在整秒边界附近（如 14999ms），向下取整会被几毫秒的网络抖动
// 带出「停一拍、跳两秒」的不均匀显示，所以四舍五入；未完成时封顶到 browse_seconds-1，避免显示满秒却没发奖，
// 完成后显示 browse_seconds。是否完成始终按真实毫秒判断，这里只影响展示。
func rewardProgressSeconds(progressMs int64, browseSeconds int, completed bool) int {
	if progressMs <= 0 {
		return 0
	}
	seconds := int((progressMs + 500) / 1000)
	if browseSeconds > 0 {
		if completed {
			return browseSeconds
		}
		if seconds > browseSeconds-1 {
			seconds = browseSeconds - 1
		}
	}
	return seconds
}

func rewardSDKHeartbeatDelta(last *time.Time, now time.Time) (int64, bool) {
	if last == nil {
		return 0, true
	}
	delta := now.Sub(*last)
	if delta < 800*time.Millisecond {
		return 0, false
	}
	if delta > 3*time.Second {
		return 0, true
	}
	return delta.Milliseconds(), true
}

type RewardSDKHeartbeatResult struct {
	Status             string  `json:"status"`
	ProgressSeconds    int     `json:"progress_seconds"`
	BrowseSeconds      int     `json:"browse_seconds"`
	Completed          bool    `json:"completed"`
	RewardAmount       float64 `json:"reward_amount"`
	HeartbeatInterval  int     `json:"heartbeat_interval"`
	IdleTimeoutSeconds int     `json:"idle_timeout_seconds"`
}

func (s *RewardTaskService) RewardSDKHeartbeat(ctx context.Context, appKey, ticket, origin, referer string) (*RewardSDKHeartbeatResult, error) {
	if err := s.rewardOpenEnabled(ctx); err != nil {
		return nil, err
	}
	partner, err := s.entClient.RewardTaskPartner.Query().Where(rewardtaskpartner.AppKeyEQ(appKey), rewardtaskpartner.EnabledEQ(true)).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_PARTNER_INVALID")
	}
	if err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	record, err := tx.RewardTaskRecord.Query().Where(rewardtaskrecord.TicketEQ(ticket)).ForUpdate().Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_TICKET_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	task, err := tx.RewardTask.Get(ctx, record.TaskID)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_TASK_UNAVAILABLE")
	}
	if err != nil {
		return nil, err
	}
	if task.PartnerID == nil || *task.PartnerID != partner.ID {
		return nil, rewardTaskError("REWARD_TICKET_PARTNER_MISMATCH")
	}
	if task.TaskType != "browse" || task.VerifyMode != "sdk" {
		return nil, rewardTaskError("REWARD_EVENT_TYPE_MISMATCH")
	}
	now := time.Now()
	if !record.TicketExpiresAt.After(now) {
		return nil, rewardTaskError("REWARD_TICKET_EXPIRED")
	}
	txService := *s
	txService.entClient = tx.Client()
	if err := txService.taskAvailable(ctx, task, now); err != nil {
		return nil, err
	}
	if !rewardSDKOriginAllowed(partner.AllowedDomains, origin, referer) {
		return nil, rewardTaskError("REWARD_ORIGIN_NOT_ALLOWED")
	}
	rewarded := false
	var completionErr error
	if record.Status != "completed" {
		millis, update := rewardSDKHeartbeatDelta(record.LastHeartbeatAt, now)
		if update {
			record, err = tx.RewardTaskRecord.UpdateOneID(record.ID).AddProgressMs(millis).SetLastHeartbeatAt(now).Save(ctx)
			if err != nil {
				return nil, err
			}
		}
		if record.ProgressMs >= int64(task.BrowseSeconds)*1000 {
			rewarded, completionErr = s.completeRewardTask(ctx, tx, record, task, now)
			if completionErr != nil && infraerrors.Reason(completionErr) != "REWARD_TASK_QUOTA_EXHAUSTED" {
				return nil, completionErr
			}
			if completionErr == nil {
				record, err = tx.RewardTaskRecord.Get(ctx, record.ID)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if rewarded {
		s.invalidateBalanceCaches(ctx, record.UserID)
	}
	// 条件 UPDATE 实现跨进程节流，心跳不写回调事件表。
	if _, err := s.entClient.RewardTaskPartner.Update().Where(rewardtaskpartner.IDEQ(partner.ID), rewardtaskpartner.Or(rewardtaskpartner.LastEventAtIsNil(), rewardtaskpartner.LastEventAtLT(now.Add(-time.Minute)))).SetLastEventAt(now).Save(ctx); err != nil {
		slog.Warn("reward SDK partner activity update failed", "partner_id", partner.ID, "error", err)
	}
	if completionErr != nil {
		return nil, completionErr
	}
	reward := task.RewardAmount
	if record.Status == "completed" {
		reward = record.RewardAmount
	}
	return &RewardSDKHeartbeatResult{Status: record.Status, ProgressSeconds: rewardProgressSeconds(record.ProgressMs, task.BrowseSeconds, record.Status == "completed"),
		BrowseSeconds: task.BrowseSeconds, Completed: record.Status == "completed", RewardAmount: reward, HeartbeatInterval: 1, IdleTimeoutSeconds: task.IdleTimeoutSeconds}, nil
}
