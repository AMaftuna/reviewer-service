package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (r *Repo) LogAssignmentsTx(ctx context.Context, tx pgx.Tx, prID string, userIDs []string, action string) error {
	for _, uid := range userIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO review_assignments(pull_request_id, assigned_user_id, action)
			VALUES($1,$2,$3)
		`, prID, uid, action)
		if err != nil {
			return err
		}
	}
	return nil
}

type UserAssignStat struct {
	UserID string `json:"user_id"`
	Count  int64  `json:"count"`
}

type PRAssignStat struct {
	PullRequestID string `json:"pull_request_id"`
	Count         int64  `json:"count"`
}

func (r *Repo) StatsByUsers(ctx context.Context) ([]UserAssignStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT assigned_user_id, COUNT(*)::bigint
		FROM review_assignments
		GROUP BY assigned_user_id
		ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []UserAssignStat
	for rows.Next() {
		var s UserAssignStat
		if err := rows.Scan(&s.UserID, &s.Count); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return res, rows.Err()
}

func (r *Repo) StatsByPRs(ctx context.Context) ([]PRAssignStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pull_request_id, COUNT(*)::bigint
		FROM review_assignments
		GROUP BY pull_request_id
		ORDER BY COUNT(*) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []PRAssignStat
	for rows.Next() {
		var s PRAssignStat
		if err := rows.Scan(&s.PullRequestID, &s.Count); err != nil {
			return nil, err
		}
		res = append(res, s)
	}
	return res, rows.Err()
}
