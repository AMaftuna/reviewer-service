package repo

import (
	"context"

	"reviewer-service/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// -------------------- Teams --------------------

func (r *Repo) TeamExistsTx(ctx context.Context, tx pgx.Tx, team string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM teams WHERE team_name=$1)`, team).Scan(&ok)
	return ok, err
}

func (r *Repo) CreateTeamTx(ctx context.Context, tx pgx.Tx, team string) error {
	_, err := tx.Exec(ctx, `INSERT INTO teams(team_name) VALUES($1)`, team)
	return err
}

func (r *Repo) GetTeam(ctx context.Context, team string) (models.Team, error) {
	var t models.Team
	err := r.pool.QueryRow(ctx, `SELECT team_name FROM teams WHERE team_name=$1`, team).Scan(&t.TeamName)
	return t, err
}

func (r *Repo) ListTeamMembers(ctx context.Context, team string) ([]models.TeamMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id, username, is_active
		FROM users WHERE team_name=$1
		ORDER BY user_id
	`, team)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.TeamMember
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, err
		}
		res = append(res, m)
	}
	return res, rows.Err()
}

// -------------------- Users --------------------

func (r *Repo) UpsertUserTx(ctx context.Context, tx pgx.Tx, id, name string, active bool, team string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO users(user_id, username, is_active, team_name)
		VALUES($1,$2,$3,$4)
		ON CONFLICT(user_id) DO UPDATE SET
			username=EXCLUDED.username,
			is_active=EXCLUDED.is_active,
			team_name=EXCLUDED.team_name
	`, id, name, active, team)
	return err
}

func (r *Repo) GetUser(ctx context.Context, id string) (models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, username, COALESCE(team_name,''), is_active
		FROM users WHERE user_id=$1
	`, id).Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive)
	return u, err
}

func (r *Repo) GetUserTx(ctx context.Context, tx pgx.Tx, id string) (models.User, error) {
	var u models.User
	err := tx.QueryRow(ctx, `
		SELECT user_id, username, COALESCE(team_name,''), is_active
		FROM users WHERE user_id=$1
	`, id).Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive)
	return u, err
}

func (r *Repo) SetIsActive(ctx context.Context, id string, active bool) (models.User, error) {
	ct, err := r.pool.Exec(ctx, `UPDATE users SET is_active=$2 WHERE user_id=$1`, id, active)
	if err != nil {
		return models.User{}, err
	}
	if ct.RowsAffected() == 0 {
		return models.User{}, pgx.ErrNoRows
	}
	return r.GetUser(ctx, id)
}

// -------------------- PRs --------------------

func (r *Repo) PRExistsTx(ctx context.Context, tx pgx.Tx, prID string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM prs WHERE pull_request_id=$1)`, prID).Scan(&ok)
	return ok, err
}

func (r *Repo) CreatePRTx(ctx context.Context, tx pgx.Tx, id, name, author string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO prs(pull_request_id, pull_request_name, author_id, status)
		VALUES($1,$2,$3,'OPEN')
	`, id, name, author)
	return err
}

func (r *Repo) InsertReviewersTx(ctx context.Context, tx pgx.Tx, prID string, reviewers []string) error {
	for i, uid := range reviewers {
		_, err := tx.Exec(ctx, `
			INSERT INTO pr_reviewers(pull_request_id, user_id, position)
			VALUES($1,$2,$3)
		`, prID, uid, i+1)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) ReplaceReviewerTx(ctx context.Context, tx pgx.Tx, prID, oldID, newID string) error {
	ct, err := tx.Exec(ctx, `
		UPDATE pr_reviewers SET user_id=$3
		WHERE pull_request_id=$1 AND user_id=$2
	`, prID, oldID, newID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repo) GetPRForUpdateTx(ctx context.Context, tx pgx.Tx, prID string) (models.PullRequest, error) {
	var pr models.PullRequest
	err := tx.QueryRow(ctx, `
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM prs WHERE pull_request_id=$1 FOR UPDATE
	`, prID).Scan(
		&pr.PullRequestID,
		&pr.PullRequestName,
		&pr.AuthorID,
		&pr.Status,
		&pr.CreatedAt,
		&pr.MergedAt,
	)
	return pr, err
}

func (r *Repo) MergePRTx(ctx context.Context, tx pgx.Tx, prID string) error {
	ct, err := tx.Exec(ctx, `
		UPDATE prs SET status='MERGED', merged_at=COALESCE(merged_at, now())
		WHERE pull_request_id=$1
	`, prID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repo) ListPRReviewerIDsTx(ctx context.Context, q querier, prID string) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT user_id FROM pr_reviewers
		WHERE pull_request_id=$1 ORDER BY position
	`, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		res = append(res, id)
	}
	return res, rows.Err()
}

func (r *Repo) ListPRReviewerIDs(ctx context.Context, prID string) ([]string, error) {
	return r.ListPRReviewerIDsTx(ctx, r.pool, prID)
}

func (r *Repo) GetPR(ctx context.Context, prID string) (models.PullRequest, error) {
	var pr models.PullRequest
	err := r.pool.QueryRow(ctx, `
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM prs WHERE pull_request_id=$1
	`, prID).Scan(
		&pr.PullRequestID,
		&pr.PullRequestName,
		&pr.AuthorID,
		&pr.Status,
		&pr.CreatedAt,
		&pr.MergedAt,
	)
	if err != nil {
		return models.PullRequest{}, err
	}

	revs, err := r.ListPRReviewerIDs(ctx, prID)
	if err != nil {
		return models.PullRequest{}, err
	}
	pr.AssignedReviewers = revs
	return pr, nil
}

func (r *Repo) ListActiveTeamUserIDsTx(ctx context.Context, tx pgx.Tx, team string, exclude []string) ([]string, error) {
	ex := map[string]struct{}{}
	for _, e := range exclude {
		ex[e] = struct{}{}
	}

	rows, err := tx.Query(ctx, `
		SELECT user_id
		FROM users
		WHERE team_name=$1 AND is_active=true
	`, team)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if _, ok := ex[id]; !ok {
			res = append(res, id)
		}
	}
	return res, rows.Err()
}

func (r *Repo) ListPRShortByReviewer(ctx context.Context, reviewer string) ([]models.PullRequestShort, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.pull_request_id, p.pull_request_name, p.author_id, p.status
		FROM pr_reviewers prr
		JOIN prs p ON p.pull_request_id=prr.pull_request_id
		WHERE prr.user_id=$1
		ORDER BY p.created_at DESC
	`, reviewer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []models.PullRequestShort
	for rows.Next() {
		var pr models.PullRequestShort
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status); err != nil {
			return nil, err
		}
		res = append(res, pr)
	}
	return res, rows.Err()
}
