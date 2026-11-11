package repository

import (
	"context"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

type redeemCodeRepository struct {
	client *dbent.Client
}

func NewRedeemCodeRepository(client *dbent.Client) service.RedeemCodeRepository {
	return &redeemCodeRepository{client: client}
}

func (r *redeemCodeRepository) Create(ctx context.Context, code *service.RedeemCode) error {
	normalizeRedeemCodeLimits(code)
	created, err := clientFromContext(ctx, r.client).RedeemCode.Create().
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays).
		SetNillableExpiresAt(code.ExpiresAt).
		SetNillableUsedBy(code.UsedBy).
		SetNillableUsedAt(code.UsedAt).
		SetNillableGroupID(code.GroupID).
		SetBenefitType(code.BenefitType).
		SetMaxUsers(code.MaxUsers).
		SetMaxUsesPerUser(code.MaxUsesPerUser).
		SetNillableParentID(code.ParentID).
		Save(ctx)
	if err == nil {
		code.ID = created.ID
		code.CreatedAt = created.CreatedAt
	}
	return err
}

func (r *redeemCodeRepository) CreateBatch(ctx context.Context, codes []service.RedeemCode) error {
	if len(codes) == 0 {
		return nil
	}

	builders := make([]*dbent.RedeemCodeCreate, 0, len(codes))
	for i := range codes {
		c := &codes[i]
		normalizeRedeemCodeLimits(c)
		b := r.client.RedeemCode.Create().
			SetCode(c.Code).
			SetType(c.Type).
			SetValue(c.Value).
			SetStatus(c.Status).
			SetNotes(c.Notes).
			SetValidityDays(c.ValidityDays).
			SetNillableExpiresAt(c.ExpiresAt).
			SetNillableUsedBy(c.UsedBy).
			SetNillableUsedAt(c.UsedAt).
			SetNillableGroupID(c.GroupID).
			SetBenefitType(c.BenefitType).
			SetMaxUsers(c.MaxUsers).
			SetMaxUsesPerUser(c.MaxUsesPerUser).
			SetNillableParentID(c.ParentID)
		builders = append(builders, b)
	}

	return r.client.RedeemCode.CreateBulk(builders...).Exec(ctx)
}

// normalizeRedeemCodeLimits 把未指定的限制补成「1 个用户、每人 1 次」，
// 即一次性兑换码的语义；只有多次兑换码会显式指定更大的值。
func normalizeRedeemCodeLimits(code *service.RedeemCode) {
	if code.MaxUsers <= 0 {
		code.MaxUsers = 1
	}
	if code.MaxUsesPerUser <= 0 {
		code.MaxUsesPerUser = 1
	}
}

func (r *redeemCodeRepository) GetByID(ctx context.Context, id int64) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.IDEQ(id)).
		WithParent().
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return redeemCodeEntityToService(m), nil
}

func (r *redeemCodeRepository) GetByCode(ctx context.Context, code string) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.CodeEQ(code)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return redeemCodeEntityToService(m), nil
}

func (r *redeemCodeRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.client.RedeemCode.Delete().Where(redeemcode.IDEQ(id)).Exec(ctx)
	return err
}

func (r *redeemCodeRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, "", "", "")
}

func (r *redeemCodeRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query()

	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}
	if status != "" {
		now := time.Now()
		switch status {
		case service.StatusExpired:
			q = q.Where(redeemcode.Or(
				redeemcode.StatusEQ(service.StatusExpired),
				redeemcode.And(
					redeemcode.StatusEQ(service.StatusUnused),
					redeemcode.ExpiresAtNotNil(),
					redeemcode.ExpiresAtLTE(now),
				),
			))
		case service.StatusUnused:
			q = q.Where(
				redeemcode.StatusEQ(service.StatusUnused),
				redeemcode.Or(
					redeemcode.ExpiresAtIsNil(),
					redeemcode.ExpiresAtGT(now),
				),
			)
		default:
			q = q.Where(redeemcode.StatusEQ(status))
		}
	}
	if search != "" {
		q = q.Where(
			redeemcode.Or(
				redeemcode.CodeContainsFold(search),
				redeemcode.HasUserWith(user.EmailContainsFold(search)),
				// 搜索多次兑换码的码值时，一并列出它的兑换流水。
				redeemcode.HasParentWith(redeemcode.CodeContainsFold(search)),
			),
		)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codesQuery := q.
		WithUser().
		WithGroup().
		WithParent().
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range redeemCodeListOrder(params) {
		codesQuery = codesQuery.Order(order)
	}

	codes, err := codesQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outCodes := redeemCodeEntitiesToService(codes)

	return outCodes, paginationResultFromTotal(int64(total), params), nil
}

func redeemCodeListOrder(params pagination.PaginationParams) []func(*entsql.Selector) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	var field string
	switch sortBy {
	case "type":
		field = redeemcode.FieldType
	case "value":
		field = redeemcode.FieldValue
	case "status":
		field = redeemcode.FieldStatus
	case "used_at":
		field = redeemcode.FieldUsedAt
	case "created_at":
		field = redeemcode.FieldCreatedAt
	case "expires_at":
		field = redeemcode.FieldExpiresAt
	case "code":
		field = redeemcode.FieldCode
	default:
		field = redeemcode.FieldID
	}

	if sortOrder == pagination.SortOrderAsc {
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(redeemcode.FieldID)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(redeemcode.FieldID)}
}

func (r *redeemCodeRepository) Update(ctx context.Context, code *service.RedeemCode) error {
	up := r.client.RedeemCode.UpdateOneID(code.ID).
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays)

	if code.UsedBy != nil {
		up.SetUsedBy(*code.UsedBy)
	} else {
		up.ClearUsedBy()
	}
	if code.UsedAt != nil {
		up.SetUsedAt(*code.UsedAt)
	} else {
		up.ClearUsedAt()
	}
	if code.GroupID != nil {
		up.SetGroupID(*code.GroupID)
	} else {
		up.ClearGroupID()
	}
	if code.ExpiresAt != nil {
		up.SetExpiresAt(*code.ExpiresAt)
	} else {
		up.ClearExpiresAt()
	}

	updated, err := up.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrRedeemCodeNotFound
		}
		return err
	}
	code.CreatedAt = updated.CreatedAt
	return nil
}

func (r *redeemCodeRepository) BatchUpdate(ctx context.Context, ids []int64, fields service.RedeemCodeBatchUpdateFields) (int64, error) {
	uniqueIDs := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return 0, nil
	}

	if tx := dbent.TxFromContext(ctx); tx != nil {
		return r.batchUpdate(ctx, tx.Client(), uniqueIDs, fields)
	}

	tx, err := r.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	defer func() { _ = tx.Rollback() }()

	updated, err := r.batchUpdate(txCtx, tx.Client(), uniqueIDs, fields)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updated, nil
}

func (r *redeemCodeRepository) batchUpdate(ctx context.Context, client *dbent.Client, ids []int64, fields service.RedeemCodeBatchUpdateFields) (int64, error) {
	existing, err := client.RedeemCode.Query().
		Where(redeemcode.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return 0, err
	}
	if len(existing) != len(ids) {
		return 0, service.ErrRedeemCodeNotFound
	}
	if fields.TouchesUsedSensitiveFields() {
		for _, code := range existing {
			if code.Status == service.StatusUsed {
				return 0, service.ErrRedeemCodeUsed
			}
		}
	}

	up := client.RedeemCode.Update().Where(redeemcode.IDIn(ids...))
	if fields.Status != nil {
		up.SetStatus(*fields.Status)
	}
	if fields.Notes != nil {
		up.SetNotes(*fields.Notes)
	}
	if fields.ExpiresAt.Set {
		if fields.ExpiresAt.Value != nil {
			up.SetExpiresAt(*fields.ExpiresAt.Value)
		} else {
			up.ClearExpiresAt()
		}
	}
	if fields.GroupID.Set {
		if fields.GroupID.Value != nil {
			up.SetGroupID(*fields.GroupID.Value)
		} else {
			up.ClearGroupID()
		}
	}

	affected, err := up.Save(ctx)
	if err != nil {
		return 0, err
	}
	if affected != len(ids) {
		return 0, service.ErrRedeemCodeNotFound
	}
	return int64(affected), nil
}

func (r *redeemCodeRepository) Use(ctx context.Context, id, userID int64) error {
	now := time.Now()
	client := clientFromContext(ctx, r.client)
	affected, err := client.RedeemCode.Update().
		Where(redeemcode.IDEQ(id), redeemcode.StatusEQ(service.StatusUnused)).
		SetStatus(service.StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrRedeemCodeUsed
	}
	return nil
}

func (r *redeemCodeRepository) RedeemMultiUse(ctx context.Context, id, userID int64, recordCode string) (*service.RedeemCode, error) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return redeemMultiUse(ctx, tx.Client(), id, userID, recordCode)
	}

	// 调用方没有开启事务时自行包一层，保证「占用名额 + 写流水」要么都生效要么都不生效。
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	record, err := redeemMultiUse(dbent.NewTxContext(ctx, tx), tx.Client(), id, userID, recordCode)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return record, nil
}

// redeemMultiUse 必须在事务内执行：中途返回错误时已占用的名额依赖回滚撤销。
func redeemMultiUse(ctx context.Context, client *dbent.Client, id, userID int64, recordCode string) (*service.RedeemCode, error) {
	now := time.Now()

	// 先占用一次兑换名额。这条条件更新同时充当行锁：并发兑换同一个码的事务在此排队，
	// 等前一个事务提交后才继续，所以下面读到的计数与该用户的已兑换次数都是已提交的结果。
	// （不用 SELECT ... FOR UPDATE：SQLite 不支持，而单元测试的库基于 SQLite。）
	affected, err := client.RedeemCode.Update().
		Where(
			redeemcode.IDEQ(id),
			redeemcode.TypeEQ(service.RedeemTypeMultiUse),
			redeemcode.StatusEQ(service.StatusUnused),
			redeemcode.Or(
				redeemcode.ExpiresAtIsNil(),
				redeemcode.ExpiresAtGT(now),
			),
		).
		AddUsedCount(1).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, multiUseUnavailableError(ctx, client, id, now)
	}

	parent, err := client.RedeemCode.Query().Where(redeemcode.IDEQ(id)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}

	userUses, err := client.RedeemCode.Query().
		Where(redeemcode.ParentIDEQ(id), redeemcode.UsedByEQ(userID)).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	if userUses >= parent.MaxUsesPerUser {
		return nil, service.ErrRedeemCodeUserLimitReached
	}

	up := client.RedeemCode.UpdateOneID(id)
	changed := false
	if userUses == 0 {
		// 该用户第一次兑换：占用一个用户名额。
		if parent.UsedUserCount >= parent.MaxUsers {
			return nil, service.ErrRedeemCodeUserQuotaExhausted
		}
		up.AddUsedUserCount(1)
		changed = true
	}
	// parent.UsedCount 已包含本次。所有用户名额的所有次数都用完后标记为已使用，
	// 之后的兑换会被上面的条件更新直接拒绝。
	if int64(parent.UsedCount) >= int64(parent.MaxUsers)*int64(parent.MaxUsesPerUser) {
		up.SetStatus(service.StatusUsed)
		changed = true
	}
	if changed {
		if err := up.Exec(ctx); err != nil {
			return nil, err
		}
	}

	// 本次兑换的流水行：类型即发放的权益类型，因此兑换历史、余额流水与充值统计
	// 都能像一次性兑换码那样直接读到它。
	created, err := client.RedeemCode.Create().
		SetCode(recordCode).
		SetType(parent.BenefitType).
		SetValue(parent.Value).
		SetStatus(service.StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(now).
		SetValidityDays(parent.ValidityDays).
		SetNillableGroupID(parent.GroupID).
		SetParentID(parent.ID).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	record := redeemCodeEntityToService(created)
	record.ParentCode = parent.Code
	return record, nil
}

// multiUseUnavailableError 解释多次兑换码为什么无法占用名额。
func multiUseUnavailableError(ctx context.Context, client *dbent.Client, id int64, now time.Time) error {
	m, err := client.RedeemCode.Query().Where(redeemcode.IDEQ(id)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrRedeemCodeNotFound
		}
		return err
	}
	code := redeemCodeEntityToService(m)
	if !code.IsMultiUse() {
		return service.ErrRedeemCodeNotFound
	}
	if code.IsExpiredAt(now) {
		return service.ErrRedeemCodeExpired
	}
	// 已用完或被禁用，与一次性兑换码不可用时的错误保持一致。
	return service.ErrRedeemCodeUsed
}

func (r *redeemCodeRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.RedeemCode, error) {
	if limit <= 0 {
		limit = 10
	}

	codes, err := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID)).
		WithGroup().
		WithParent().
		Order(dbent.Desc(redeemcode.FieldUsedAt), dbent.Desc(redeemcode.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return redeemCodeEntitiesToService(codes), nil
}

// ListByUserPaginated returns paginated balance/concurrency history for a user.
// Supports optional type filter (e.g. "balance", "admin_balance", "concurrency", "admin_concurrency", "subscription").
func (r *redeemCodeRepository) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID))

	// Optional type filter
	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codes, err := q.
		WithGroup().
		WithParent().
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(redeemcode.FieldUsedAt), dbent.Desc(redeemcode.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return redeemCodeEntitiesToService(codes), paginationResultFromTotal(int64(total), params), nil
}

// SumPositiveBalanceByUser returns total recharged amount (sum of value > 0 where type is balance/admin_balance).
func (r *redeemCodeRepository) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	var result []struct {
		Sum float64 `json:"sum"`
	}
	err := r.client.RedeemCode.Query().
		Where(
			redeemcode.UsedByEQ(userID),
			redeemcode.ValueGT(0),
			redeemcode.TypeIn("balance", "admin_balance"),
		).
		Aggregate(dbent.As(dbent.Sum(redeemcode.FieldValue), "sum")).
		Scan(ctx, &result)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Sum, nil
}

func redeemCodeEntityToService(m *dbent.RedeemCode) *service.RedeemCode {
	if m == nil {
		return nil
	}
	out := &service.RedeemCode{
		ID:           m.ID,
		Code:         m.Code,
		Type:         m.Type,
		Value:        m.Value,
		Status:       m.Status,
		UsedBy:       m.UsedBy,
		UsedAt:       m.UsedAt,
		Notes:        derefString(m.Notes),
		CreatedAt:    m.CreatedAt,
		ExpiresAt:    m.ExpiresAt,
		GroupID:      m.GroupID,
		ValidityDays: m.ValidityDays,

		BenefitType:    m.BenefitType,
		MaxUsers:       m.MaxUsers,
		MaxUsesPerUser: m.MaxUsesPerUser,
		UsedCount:      m.UsedCount,
		UsedUserCount:  m.UsedUserCount,
		ParentID:       m.ParentID,
	}
	if m.Edges.Parent != nil {
		out.ParentCode = m.Edges.Parent.Code
	}
	if m.Edges.User != nil {
		out.User = userEntityToService(m.Edges.User)
	}
	if m.Edges.Group != nil {
		out.Group = groupEntityToService(m.Edges.Group)
	}
	return out
}

func redeemCodeEntitiesToService(models []*dbent.RedeemCode) []service.RedeemCode {
	out := make([]service.RedeemCode, 0, len(models))
	for i := range models {
		if s := redeemCodeEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}
