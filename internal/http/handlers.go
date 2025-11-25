package httpx

import (
	"encoding/json"
	"net/http"

	"reviewer-service/internal/models"
	"reviewer-service/internal/service"
)

type Handlers struct {
	svc *service.Service
}

// -------- Teams --------

func (h *Handlers) TeamAdd(w http.ResponseWriter, r *http.Request) {
	var req TeamAddReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TeamName == "" {
		writeErr(w, 400, "NOT_FOUND", "invalid body")
		return
	}

	members := make([]models.TeamMember, 0, len(req.Members))
	for _, m := range req.Members {
		members = append(members, models.TeamMember{
			UserID:   m.UserID,
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}

	team, err := h.svc.TeamAdd(r.Context(), req.TeamName, members)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"team": team})
}

func (h *Handlers) TeamGet(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeErr(w, 400, "NOT_FOUND", "team_name required")
		return
	}
	team, err := h.svc.TeamGet(r.Context(), teamName)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, 200, team)
}

func (h *Handlers) TeamDeactivate(w http.ResponseWriter, r *http.Request) {
	var req TeamDeactivateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TeamName == "" {
		writeErr(w, 400, "NOT_FOUND", "invalid body")
		return
	}
	res, err := h.svc.TeamDeactivate(r.Context(), req.TeamName, req.UserIDs)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, 200, res)
}

// -------- Users --------

func (h *Handlers) UserSetIsActive(w http.ResponseWriter, r *http.Request) {
	var req SetIsActiveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeErr(w, 400, "NOT_FOUND", "invalid body")
		return
	}
	user, err := h.svc.UserSetIsActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"user": user})
}

func (h *Handlers) UserGetReview(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		writeErr(w, 400, "NOT_FOUND", "user_id required")
		return
	}
	list, err := h.svc.UserGetReview(r.Context(), uid)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"user_id":       uid,
		"pull_requests": list,
	})
}

// -------- PRs --------

func (h *Handlers) PRCreate(w http.ResponseWriter, r *http.Request) {
	var req PRCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		req.PullRequestID == "" || req.PullRequestName == "" || req.AuthorID == "" {
		writeErr(w, 400, "NOT_FOUND", "invalid body")
		return
	}

	pr, err := h.svc.PRCreate(r.Context(), req.PullRequestID, req.PullRequestName, req.AuthorID)

	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"pr": pr})
}

func (h *Handlers) PRMerge(w http.ResponseWriter, r *http.Request) {
	var req PRMergeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PullRequestID == "" {
		writeErr(w, 400, "NOT_FOUND", "invalid body")
		return
	}
	pr, err := h.svc.PRMerge(r.Context(), req.PullRequestID)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"pr": pr})
}

func (h *Handlers) PRReassign(w http.ResponseWriter, r *http.Request) {
	var req PRReassignReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		req.PullRequestID == "" || req.OldUserID == "" {
		writeErr(w, 400, "NOT_FOUND", "invalid body")
		return
	}
	pr, replacedBy, err := h.svc.PRReassign(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"pr": pr, "replaced_by": replacedBy})
}

// -------- Stats --------

func (h *Handlers) StatsGet(w http.ResponseWriter, r *http.Request) {
	by := r.URL.Query().Get("by")
	if by == "" {
		by = "users"
	}

	switch by {
	case "users":
		st, err := h.svc.StatsByUsers(r.Context())
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"by_users": st})
	case "prs":
		st, err := h.svc.StatsByPRs(r.Context())
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"by_prs": st})
	default:
		writeErr(w, 400, "NOT_FOUND", "unknown by param")
	}
}

// -------- error helpers --------

func writeSvcErr(w http.ResponseWriter, err error) {
	code, msg, httpCode := service.ToHTTPError(err)
	writeErr(w, httpCode, code, msg)
}

func writeErr(w http.ResponseWriter, httpCode int, code, msg string) {
	writeJSON(w, httpCode, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": msg,
		},
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
