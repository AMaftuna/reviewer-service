package repo

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (r *Repo) DeactivateUsersTx(ctx context.Context, tx pgx.Tx, team string, userIDs []string) ([]string, error) {
	if len(userIDs) == 0 {
		rows, err := tx.Query(ctx, `
			UPDATE users SET is_active=false
			WHERE team_name=$1 AND is_active=true
			RETURNING user_id
		`, team)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}

	rows, err := tx.Query(ctx, `
		UPDATE users SET is_active=false
		WHERE team_name=$1 AND user_id = ANY($2) AND is_active=true
		RETURNING user_id
	`, team, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type AffectedPR struct {
	PRID   string
	OldUID string
	Author string
}

func (r *Repo) FindAffectedOpenPRsTx(ctx context.Context, tx pgx.Tx, deactivated []string) ([]AffectedPR, error) {
	rows, err := tx.Query(ctx, `
		SELECT prr.pull_request_id, prr.user_id, p.author_id
		FROM pr_reviewers prr
		JOIN prs p ON p.pull_request_id=prr.pull_request_id
		WHERE p.status='OPEN' AND prr.user_id = ANY($1)
	`, deactivated)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []AffectedPR
	for rows.Next() {
		var a AffectedPR
		if err := rows.Scan(&a.PRID, &a.OldUID, &a.Author); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

func (r *Repo) DeleteReviewerTx(ctx context.Context, tx pgx.Tx, prID, userID string) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM pr_reviewers WHERE pull_request_id=$1 AND user_id=$2
	`, prID, userID)
	return err
}
