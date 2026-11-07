//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9" //nolint:depguard // 测试使用 miniredis 验证真实 Redis 限流协议。
	"github.com/stretchr/testify/require"
)

func newTicketCoreTestService(t *testing.T) (*TicketService, sqlmock.Sqlmock, *miniredis.Miniredis) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mock.ExpectationsWereMet()); _ = db.Close() })
	settings := NewSettingService(newNotificationEmailMemorySettingRepo(), nil)
	enabled := true
	_, err = settings.UpdateTicketSettings(t.Context(), TicketSettingsUpdate{Enabled: &enabled})
	require.NoError(t, err)
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return NewTicketService(db, settings, middleware.NewRateLimiter(client), nil), mock, mini
}

func TestTicketEnumsUnicodeAndNumber(t *testing.T) {
	for _, value := range []string{"general", "billing", "technical", "account", "feature", "other"} {
		require.True(t, validTicketCategory(value))
	}
	for _, value := range []string{"low", "normal", "high", "urgent"} {
		require.True(t, validTicketPriority(value))
	}
	for _, value := range []string{"pending_admin", "pending_user", "resolved", "closed"} {
		require.True(t, validTicketStatus(value))
	}
	require.False(t, validTicketStatus("open"))
	require.False(t, validTicketPriority("critical"))
	require.False(t, validTicketCategory("private"))
	require.True(t, validateText("中文🙂", 3))
	require.False(t, validateText("中文🙂", 2))
	require.False(t, validateText(string([]byte{0xff}), 3))
	require.False(t, validateText("", 1))
	require.False(t, validateText("body\x00text", 100))
	seen := map[string]bool{}
	for range 100 {
		number, err := (&TicketService{}).ticketNo()
		require.NoError(t, err)
		require.Regexp(t, `^TK[0-9]{8}[A-Z0-9]{6}$`, number)
		require.False(t, seen[number])
		seen[number] = true
	}
}

func TestTicketJSONContractAndMessagePrivacy(t *testing.T) {
	id := int64(9)
	message := ticketMessageVisibility(TicketMessage{ID: 1, TicketID: 2, AuthorType: "admin", AuthorID: &id, AuthorName: "private-agent", Via: "agent", Body: "plain"}, false)
	require.Nil(t, message.AuthorID)
	require.Equal(t, "Support", message.AuthorName)
	encoded, err := json.Marshal(message)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Len(t, fields, 9)
	require.Contains(t, fields, "author_id")
	require.Nil(t, fields["author_id"])
	require.NotContains(t, string(encoded), "private-agent")
	admin := AdminTicketSummary{TicketSummary: TicketSummary{ID: 2}, UserEmail: "user@example.com"}
	encoded, err = json.Marshal(admin)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &fields))
	for _, name := range []string{"id", "ticket_no", "subject", "category", "priority", "status", "user_id", "assignee_id", "unread", "message_count", "last_message_at", "last_message_by", "created_at", "updated_at", "closed_at", "user_email", "user_username", "assignee_name", "user_unread", "admin_unread"} {
		require.Contains(t, fields, name)
	}
}

func TestTicketGetHidesOtherOwners(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			svc, mock, _ := newTicketCoreTestService(t)
			mock.ExpectBegin()
			query := mock.ExpectQuery("SELECT user_id FROM tickets WHERE id=\\$1 FOR UPDATE").WithArgs(int64(2))
			if missing {
				query.WillReturnError(sql.ErrNoRows)
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(99))
			}
			mock.ExpectRollback()
			_, err := svc.GetTicket(t.Context(), 1, 2, false, 0)
			require.Equal(t, "TICKET_NOT_FOUND", infraerrors.Reason(err))
		})
	}
}

func TestTicketGetFiltersInternalAndClearsUnreadAtomically(t *testing.T) {
	svc, mock, _ := newTicketCoreTestService(t)
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id FROM tickets.*FOR UPDATE").WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(1))
	mock.ExpectQuery("SELECT m.id.*AND NOT m.internal ORDER BY m.id").WithArgs(int64(2), int64(0)).WillReturnRows(sqlmock.NewRows([]string{"id", "ticket_id", "author_type", "author_id", "via", "internal", "body", "created_at", "name"}).AddRow(3, 2, "admin", 9, "agent", false, "Hello", now, "private-agent"))
	mock.ExpectExec("UPDATE tickets SET user_unread=false WHERE id").WithArgs(int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT t.id,t.ticket_no.*FROM tickets t WHERE t.id").WithArgs(int64(2)).WillReturnRows(ticketTestSummaryRows(now).AddRow(2, "TK20261106ABCDEF", "subject", "general", "normal", "pending_user", 1, nil, false, 2, now, "admin", now, now, nil))
	mock.ExpectCommit()
	detail, err := svc.GetTicket(t.Context(), 1, 2, false, 0)
	require.NoError(t, err)
	require.False(t, detail.Ticket.Unread)
	require.Len(t, detail.Messages, 1)
	require.Equal(t, "Support", detail.Messages[0].AuthorName)
	require.Nil(t, detail.Messages[0].AuthorID)
}

func ticketTestSummaryRows(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "ticket_no", "subject", "category", "priority", "status", "user_id", "assignee_id", "unread", "message_count", "last_message_at", "last_message_by", "created_at", "updated_at", "closed_at"})
}

func TestTicketMessageStateTransitionsAndInternalNotes(t *testing.T) {
	for _, tc := range []struct {
		name, before, author, update string
		admin, internal              bool
	}{
		{"user reply", "pending_user", "user", "admin_unread=true", false, false},
		{"user revives resolved", "resolved", "user", "admin_unread=true", false, false},
		{"admin public reply", "pending_admin", "admin", "user_unread=true", true, false},
		{"admin revives closed", "closed", "admin", "user_unread=true", true, false},
		{"internal note does not mutate public metadata", "resolved", "admin", "", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, mock, _ := newTicketCoreTestService(t)
			mock.ExpectBegin()
			if !tc.admin {
				mock.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("SELECT user_id,status FROM tickets.*FOR UPDATE").WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"user_id", "status"}).AddRow(1, tc.before))
			if !tc.admin && tc.before == "resolved" {
				mock.ExpectQuery("SELECT COUNT.*FROM tickets").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			}
			mock.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("INSERT INTO ticket_messages").WithArgs(int64(2), tc.author, int64(1), "agent", nil, tc.internal, "body", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
			if !tc.internal {
				status := "pending_admin"
				if tc.admin {
					status = "pending_user"
				}
				mock.ExpectExec("UPDATE tickets SET status=.*"+tc.update).WithArgs(int64(2), status, sqlmock.AnyArg(), tc.author).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("SELECT COALESCE.*FROM users").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("actor"))
			mock.ExpectCommit()
			message, err := svc.AddMessage(t.Context(), 1, 2, tc.admin, "agent", nil, TicketMessageInput{Body: "body", Internal: tc.internal})
			require.NoError(t, err)
			require.Equal(t, tc.author, message.AuthorType)
			require.Equal(t, tc.internal, message.Internal)
		})
	}
}

func TestTicketMessageRejectsClosedAndCrossUser(t *testing.T) {
	for _, tc := range []struct {
		owner          int64
		status, reason string
	}{{1, "closed", "TICKET_CLOSED"}, {99, "pending_user", "TICKET_NOT_FOUND"}} {
		svc, mock, _ := newTicketCoreTestService(t)
		mock.ExpectBegin()
		mock.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT user_id,status FROM tickets.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"owner", "status"}).AddRow(tc.owner, tc.status))
		mock.ExpectRollback()
		_, err := svc.AddMessage(t.Context(), 1, 2, false, "web", nil, TicketMessageInput{Body: "body"})
		require.Equal(t, tc.reason, infraerrors.Reason(err))
	}
}

func TestTicketReopenTransitionsAndOpenLimit(t *testing.T) {
	for _, tc := range []struct {
		status, reason string
		open           int
	}{{"pending_admin", "TICKET_INVALID_TRANSITION", 0}, {"pending_user", "TICKET_INVALID_TRANSITION", 0}, {"resolved", "TICKET_OPEN_LIMIT", 5}, {"closed", "TICKET_OPEN_LIMIT", 5}} {
		t.Run(tc.status, func(t *testing.T) {
			svc, mock, _ := newTicketCoreTestService(t)
			mock.ExpectBegin()
			mock.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery("SELECT user_id,status FROM tickets.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"owner", "status"}).AddRow(1, tc.status))
			if tc.open > 0 {
				mock.ExpectQuery("SELECT COUNT.*FROM tickets").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.open))
			}
			mock.ExpectRollback()
			_, err := svc.CloseReopen(t.Context(), 1, 2, true)
			require.Equal(t, tc.reason, infraerrors.Reason(err))
		})
	}
}

func TestTicketBusinessLimitsSharedAcrossWebAndAgentAndFailClosed(t *testing.T) {
	svc, _, mini := newTicketCoreTestService(t)
	for range 2 {
		require.NoError(t, svc.businessLimit(t.Context(), "create:1", 2, time.Hour, "TICKET_CREATE_RATE_LIMITED"))
	}
	err := svc.businessLimit(t.Context(), "create:1", 2, time.Hour, "TICKET_CREATE_RATE_LIMITED")
	require.Equal(t, "TICKET_CREATE_RATE_LIMITED", infraerrors.Reason(err))
	mini.FastForward(time.Hour)
	require.NoError(t, svc.businessLimit(t.Context(), "create:1", 2, time.Hour, "TICKET_CREATE_RATE_LIMITED"))
	require.NoError(t, svc.businessLimit(t.Context(), "message:1", 1, time.Minute, "TICKET_MESSAGE_RATE_LIMITED"))
	err = svc.businessLimit(t.Context(), "message:1", 1, time.Minute, "TICKET_MESSAGE_RATE_LIMITED")
	require.Equal(t, "TICKET_MESSAGE_RATE_LIMITED", infraerrors.Reason(err))
	mini.Close()
	err = svc.businessLimit(context.Background(), "create:1", 2, time.Hour, "TICKET_CREATE_RATE_LIMITED")
	require.Equal(t, "AGENT_RATE_LIMIT_UNAVAILABLE", infraerrors.Reason(err))
}

func TestTicketListScopesAndLiteralKeyword(t *testing.T) {
	where, args, err := ticketListWhere(TicketListFilter{Keyword: "a%_\\", UserID: 99}, false, 1)
	require.NoError(t, err)
	require.Contains(t, where, "t.user_id=$1")
	require.Equal(t, int64(1), args[0])
	require.Equal(t, `%a\%\_\\%`, args[1])
	require.NotContains(t, where, "search_user")
	where, _, err = ticketListWhere(TicketListFilter{Keyword: "alice@example.com"}, true, 0)
	require.NoError(t, err)
	require.Contains(t, where, "search_user.email ILIKE")
	_, _, err = ticketListWhere(TicketListFilter{Status: "pending_user' OR 1=1"}, true, 0)
	require.Equal(t, "TICKET_INVALID_INPUT", infraerrors.Reason(err))
	require.False(t, strings.Contains(where, "alice@example.com"))
}
