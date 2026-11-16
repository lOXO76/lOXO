package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type creationCostRepository struct {
	db *sql.DB
}

func NewCreationCostRepository(db *sql.DB) service.CreationCostRepository {
	return &creationCostRepository{db: db}
}

func (r *creationCostRepository) SumUserRequestCosts(ctx context.Context, userID int64, requestIDs []string) (map[string]float64, error) {
	if len(requestIDs) == 0 {
		return map[string]float64{}, nil
	}
	args := []any{userID}
	placeholders := make([]string, len(requestIDs))
	for i, id := range requestIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, id)
	}
	return r.queryCosts(ctx, `SELECT request_id, SUM(actual_cost) FROM usage_logs
 WHERE user_id = $1 AND request_id IN (`+strings.Join(placeholders, ",")+`) GROUP BY request_id`, args...)
}

// SumUserTaskCosts batches one page into a single SQL query. Video money events
// use a stable task ID for every protocol. MiniMax file aliases resolve through
// the durable reservation, scoped to the same user, key and protocol.
func (r *creationCostRepository) SumUserTaskCosts(ctx context.Context, userID int64, records []service.AsyncTaskRecord) (map[string]float64, error) {
	values := make([]string, 0, len(records))
	args := []any{userID}
	for _, task := range records {
		if task.UserID != userID {
			continue
		}
		requestID := task.RequestID
		switch task.Kind {
		case "image":
			// Historical image records contain only imgtask_ IDs, which were
			// never billing IDs. Do not invent a correlation for those records.
			requestID = task.BillingRequestID
			if requestID == "" {
				continue
			}
		case "video":
			if requestID == "" {
				continue
			}
		default:
			continue
		}
		n := len(args) + 1
		values = append(values, fmt.Sprintf("(CAST($%d AS text),CAST($%d AS bigint),CAST($%d AS text),CAST($%d AS text),CAST($%d AS text))", n, n+1, n+2, n+3, n+4))
		args = append(args, task.ID, task.APIKeyID, requestID, task.Protocol, task.Kind)
	}
	if len(values) == 0 {
		return map[string]float64{}, nil
	}
	query := `WITH owner(user_id) AS (VALUES (CAST($1 AS bigint))),
 requested(id, api_key_id, request_id, protocol, kind) AS (VALUES ` + strings.Join(values, ",") + `),
 file_tasks AS (
 SELECT q.id, q.api_key_id, v.task_id,
   ROW_NUMBER() OVER (PARTITION BY q.id ORDER BY v.created_at DESC, v.id DESC) AS match_rank
 FROM requested q JOIN video_billing_reservations v ON v.user_id = (SELECT user_id FROM owner)
   AND v.api_key_id = q.api_key_id AND v.protocol = q.protocol
   AND v.file_id = SUBSTR(q.request_id, 14)
 WHERE q.kind = 'video' AND q.request_id LIKE 'minimax-file:%'
 ),
 billing_keys AS (
 SELECT id, api_key_id, CASE WHEN kind = 'image' THEN request_id
   WHEN request_id LIKE 'grok-video:%' THEN request_id ELSE 'grok-video:' || request_id END AS request_id
 FROM requested WHERE kind = 'image' OR request_id NOT LIKE 'minimax-file:%'
 UNION
 SELECT id, api_key_id, 'grok-video:' || task_id
 FROM file_tasks WHERE match_rank = 1 AND task_id <> ''
 )
 SELECT k.id, SUM(u.actual_cost) FROM billing_keys k JOIN usage_logs u
 ON u.user_id = (SELECT user_id FROM owner) AND u.api_key_id = k.api_key_id AND u.request_id = k.request_id
 GROUP BY k.id`
	return r.queryCosts(ctx, query, args...)
}

func (r *creationCostRepository) queryCosts(ctx context.Context, query string, args ...any) (map[string]float64, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	costs := make(map[string]float64)
	for rows.Next() {
		var id string
		var cost float64
		if err := rows.Scan(&id, &cost); err != nil {
			return nil, err
		}
		costs[id] = cost
	}
	return costs, rows.Err()
}
