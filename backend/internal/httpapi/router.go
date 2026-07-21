// Package httpapi wires chi routes and middleware for the public API.
package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/novel2av/backend/internal/service"
)

// NewRouter returns the top-level HTTP handler. All endpoints live under /api/v1.
func NewRouter(svcs *service.Services) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(corsMiddleware)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", readyz(svcs))

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/projects", func(r chi.Router) {
			r.Post("/", createProject(svcs))
			r.Get("/", listProjects(svcs))
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", getProject(svcs))
				r.Delete("/", deleteProject(svcs))
				r.Post("/pipeline:run", runPipeline(svcs))
				r.Post("/pipeline:rerun", rerunPipeline(svcs))
				r.Get("/chapters", listChapters(svcs))
				r.Post("/chapters:split", splitChapters(svcs))
				r.Get("/characters", listCharacters(svcs))
				r.Post("/characters:extract", extractCharacters(svcs))
				r.Get("/shots", listShots(svcs))
			})
		})
		r.Route("/chapters/{id}", func(r chi.Router) {
			r.Patch("/", patchChapter(svcs))
			r.Post("/video:compose", composeChapterVideo(svcs))
		})
		r.Route("/characters/{id}", func(r chi.Router) {
			r.Post("/image:regen", regenCharacterImage(svcs))
		})
		r.Route("/shots/{id}", func(r chi.Router) {
			r.Post("/image:regen", regenShotImage(svcs))
			r.Post("/tts:regen", regenShotTTS(svcs))
		})
		r.Get("/jobs/{id}", getJob(svcs))
		r.Get("/assets/{id}", getAsset(svcs))
		r.Get("/ws/projects/{id}", wsProject(svcs))
	})

	return r
}
