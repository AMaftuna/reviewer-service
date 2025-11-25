package service

import (
	"context"
	crand "crypto/rand"
	"errors"
	"math/big"

	"reviewer-service/internal/models"
	"reviewer-service/internal/repo"

	"github.com/jackc/pgx/v5"
)

var (
	ErrTeamExists  = errors.New("TEAM_EXISTS")
	ErrPRExists    = errors.New("PR_EXISTS")
	ErrPRMerged    = errors.New("PR_MERGED")
	ErrNotAssigned = errors.New("NOT_ASSIGNED")
	ErrNoCandidate = errors.New("NO_CANDIDATE")
	ErrNotFound    = errors.New("NOT_FOUND")
)

type Service struct {
	r *repo.Repo
}

func New(r *repo.Repo) *Service { return &Service{r: r} }

// -------- Teams --------

func (s *Service) TeamAdd(ctx context.Context, teamName string, members []models.TeamMember) (models.Team, error) {
	tx, err := s.r.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.Team{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	exists, err := s.r.TeamExistsTx(ctx, tx, teamName)
	if err != nil {
		return models.Team{}, err
	}
	if exists {
		return models.Team{}, ErrTeamExists
	}

	if err := s.r.CreateTeamTx(ctx, tx, teamName); err != nil {
		return models.Team{}, err
	}

	for _, m := range members {
		if m.UserID == "" || m.Username == "" {
			return models.Team{}, ErrNotFound
		}
		if err := s.r.UpsertUserTx(ctx, tx, m.UserID, m.Username, m.IsActive, teamName); err != nil {
			return models.Team{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Team{}, err
	}

	return s.TeamGet(ctx, teamName)
}

func (s *Service) TeamGet(ctx context.Context, teamName string) (models.Team, error) {
	t, err := s.r.GetTeam(ctx, teamName)
	if err != nil {
		return models.Team{}, ErrNotFound
	}
	mm, err := s.r.ListTeamMembers(ctx, teamName)
	if err != nil {
		return models.Team{}, err
	}
	t.Members = mm
	return t, nil
}

func (s *Service) TeamDeactivate(ctx context.Context, team string, userIDs []string) (map[string]any, error) {
	tx, err := s.r.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := s.r.GetTeam(ctx, team); err != nil {
		return nil, ErrNotFound
	}

	deactivated, err := s.r.DeactivateUsersTx(ctx, tx, team, userIDs)
	if err != nil {
		return nil, err
	}

	affected, err := s.r.FindAffectedOpenPRsTx(ctx, tx, deactivated)
	if err != nil {
		return nil, err
	}

	reassigned := 0
	removed := 0

	for _, a := range affected {
		oldUser, err := s.r.GetUserTx(ctx, tx, a.OldUID)
		if err != nil || oldUser.TeamName == "" {
			_ = s.r.DeleteReviewerTx(ctx, tx, a.PRID, a.OldUID)
			removed++
			continue
		}

		current, _ := s.r.ListPRReviewerIDsTx(ctx, tx, a.PRID)
		exclude := append([]string{a.Author, a.OldUID}, current...)

		cands, err := s.r.ListActiveTeamUserIDsTx(ctx, tx, oldUser.TeamName, exclude)
		if err != nil {
			return nil, err
		}

		if len(cands) == 0 {
			if err := s.r.DeleteReviewerTx(ctx, tx, a.PRID, a.OldUID); err != nil {
				return nil, err
			}
			removed++
			continue
		}

		newID := pickNRandom(cands, 1)[0]
		if err := s.r.ReplaceReviewerTx(ctx, tx, a.PRID, a.OldUID, newID); err != nil {
			return nil, err
		}
		if err := s.r.LogAssignmentsTx(ctx, tx, a.PRID, []string{newID}, "SAFE_REASSIGN"); err != nil {
			return nil, err
		}
		reassigned++
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return map[string]any{
		"team_name":   team,
		"deactivated": deactivated,
		"safe_reassign": map[string]int{
			"reassigned": reassigned,
			"removed":    removed,
		},
	}, nil
}

// -------- Users --------

func (s *Service) UserSetIsActive(ctx context.Context, userID string, active bool) (models.User, error) {
	u, err := s.r.SetIsActive(ctx, userID, active)
	if err != nil {
		return models.User{}, ErrNotFound
	}
	return u, nil
}

func (s *Service) UserGetReview(ctx context.Context, userID string) ([]models.PullRequestShort, error) {
	_, err := s.r.GetUser(ctx, userID)
	if err != nil {
		return nil, ErrNotFound
	}
	return s.r.ListPRShortByReviewer(ctx, userID)
}

// -------- PRs --------

func (s *Service) PRCreate(ctx context.Context, prID, prName, authorID string) (models.PullRequest, error) {
	tx, err := s.r.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.PullRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	author, err := s.r.GetUserTx(ctx, tx, authorID)
	if err != nil || author.TeamName == "" {
		return models.PullRequest{}, ErrNotFound
	}

	if ok, _ := s.r.PRExistsTx(ctx, tx, prID); ok {
		return models.PullRequest{}, ErrPRExists
	}

	cands, err := s.r.ListActiveTeamUserIDsTx(ctx, tx, author.TeamName, []string{author.UserID})
	if err != nil {
		return models.PullRequest{}, err
	}
	revs := pickNRandom(cands, 2)

	if err := s.r.CreatePRTx(ctx, tx, prID, prName, authorID); err != nil {
		return models.PullRequest{}, err
	}
	if err := s.r.InsertReviewersTx(ctx, tx, prID, revs); err != nil {
		return models.PullRequest{}, err
	}
	if err := s.r.LogAssignmentsTx(ctx, tx, prID, revs, "AUTO_ASSIGN"); err != nil {
		return models.PullRequest{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.PullRequest{}, err
	}

	return s.r.GetPR(ctx, prID)
}

func (s *Service) PRMerge(ctx context.Context, prID string) (models.PullRequest, error) {
	tx, err := s.r.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.PullRequest{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pr, err := s.r.GetPRForUpdateTx(ctx, tx, prID)
	if err != nil {
		return models.PullRequest{}, ErrNotFound
	}

	if pr.Status == models.PRMerged {
		_ = tx.Commit(ctx)
		return s.r.GetPR(ctx, prID)
	}

	if err := s.r.MergePRTx(ctx, tx, prID); err != nil {
		return models.PullRequest{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.PullRequest{}, err
	}
	return s.r.GetPR(ctx, prID)
}

func (s *Service) PRReassign(ctx context.Context, prID, oldUserID string) (models.PullRequest, string, error) {
	tx, err := s.r.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.PullRequest{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pr, err := s.r.GetPRForUpdateTx(ctx, tx, prID)
	if err != nil {
		return models.PullRequest{}, "", ErrNotFound
	}
	if pr.Status == models.PRMerged {
		return models.PullRequest{}, "", ErrPRMerged
	}

	reviewers, err := s.r.ListPRReviewerIDsTx(ctx, tx, prID)
	if err != nil {
		return models.PullRequest{}, "", err
	}

	found := false
	var others []string
	for _, id := range reviewers {
		if id == oldUserID {
			found = true
		} else {
			others = append(others, id)
		}
	}
	if !found {
		return models.PullRequest{}, "", ErrNotAssigned
	}

	oldUser, err := s.r.GetUserTx(ctx, tx, oldUserID)
	if err != nil || oldUser.TeamName == "" {
		return models.PullRequest{}, "", ErrNotFound
	}

	exclude := append([]string{pr.AuthorID, oldUserID}, others...)
	cands, err := s.r.ListActiveTeamUserIDsTx(ctx, tx, oldUser.TeamName, exclude)
	if err != nil {
		return models.PullRequest{}, "", err
	}
	if len(cands) == 0 {
		return models.PullRequest{}, "", ErrNoCandidate
	}

	newID := pickNRandom(cands, 1)[0]
	if err := s.r.ReplaceReviewerTx(ctx, tx, prID, oldUserID, newID); err != nil {
		return models.PullRequest{}, "", err
	}
	if err := s.r.LogAssignmentsTx(ctx, tx, prID, []string{newID}, "REASSIGN"); err != nil {
		return models.PullRequest{}, "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.PullRequest{}, "", err
	}

	updated, err := s.r.GetPR(ctx, prID)
	return updated, newID, err
}

// -------- Stats --------

func (s *Service) StatsByUsers(ctx context.Context) ([]repo.UserAssignStat, error) {
	return s.r.StatsByUsers(ctx)
}

func (s *Service) StatsByPRs(ctx context.Context) ([]repo.PRAssignStat, error) {
	return s.r.StatsByPRs(ctx)
}

// -------- helpers --------

func pickNRandom(ids []string, n int) []string {
	if n <= 0 || len(ids) == 0 {
		return nil
	}
	if len(ids) <= n {
		out := append([]string{}, ids...)
		return out
	}
	out := append([]string{}, ids...)
	for i := len(out) - 1; i > 0; i-- {
		j := cryptoRandInt(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out[:n]
}

func cryptoRandInt(upper int) int {
	if upper <= 1 {
		return 0
	}
	nBig, _ := crand.Int(crand.Reader, big.NewInt(int64(upper)))
	return int(nBig.Int64())
}

func ToHTTPError(err error) (code string, msg string, httpCode int) {
	switch {
	case errors.Is(err, ErrTeamExists):
		return "TEAM_EXISTS", "team_name already exists", 400
	case errors.Is(err, ErrPRExists):
		return "PR_EXISTS", "PR id already exists", 409
	case errors.Is(err, ErrPRMerged):
		return "PR_MERGED", "cannot reassign on merged PR", 409
	case errors.Is(err, ErrNotAssigned):
		return "NOT_ASSIGNED", "reviewer is not assigned to this PR", 409
	case errors.Is(err, ErrNoCandidate):
		return "NO_CANDIDATE", "no active replacement candidate in team", 409
	case errors.Is(err, ErrNotFound):
		return "NOT_FOUND", "resource not found", 404
	default:
		return "NOT_FOUND", "internal error", 500
	}
}
