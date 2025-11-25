package httpx

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"reviewer-service/internal/service"
)

func NewRouter(svc *service.Service) http.Handler {
	h := &Handlers{svc: svc}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(5 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// Teams
	r.Post("/team/add", h.TeamAdd)
	r.Get("/team/get", h.TeamGet)
	r.Post("/team/deactivate", h.TeamDeactivate)

	// Users
	r.Post("/users/setIsActive", h.UserSetIsActive)
	r.Get("/users/getReview", h.UserGetReview)

	// PRs
	r.Post("/pullRequest/create", h.PRCreate)
	r.Post("/pullRequest/merge", h.PRMerge)
	r.Post("/pullRequest/reassign", h.PRReassign)

	// Stats
	r.Get("/stats/get", h.StatsGet)

	return r
}
