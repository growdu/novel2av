package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/novel2av/backend/internal/service"
)

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
				r.Post("/chapters:ingest", ingestChapters(svcs))
				r.Get("/characters", listCharacters(svcs))
				r.Post("/characters:extract", extractCharacters(svcs))
				r.Post("/characters:ingest", ingestCharacters(svcs))
				r.Get("/shots", listShots(svcs))
				r.Post("/shots:breakdown", triggerBreakdown(svcs))
				r.Post("/shots:breakdown:ingest", ingestBreakdown(svcs))
			})
		})
		r.Route("/chapters/{id}", func(r chi.Router) {
			r.Patch("/", patchChapter(svcs))
			r.Get("/video", getChapterVideo(svcs))
			r.Post("/video:compose", composeChapterVideo(svcs))
			r.Post("/video:ingest", ingestChapterVideo(svcs))
		})
		r.Route("/projects/{id}/videos", func(r chi.Router) {
			r.Get("/", listChapterVideos(svcs))
			r.Post("/compose", composeAllChapters(svcs))
		})
		r.Route("/projects/{id}/full", func(r chi.Router) {
			r.Get("/", getProjectVideo(svcs))
			r.Post("/compose", composeProjectVideo(svcs))
			r.Post("/ingest", ingestProjectVideo(svcs))
		})
		r.Route("/characters/{id}", func(r chi.Router) {
			r.Get("/", getCharacter(svcs))
			r.Patch("/", patchCharacter(svcs))
			r.Delete("/", deleteCharacter(svcs))
			r.Post("/image:regen", regenCharacterImage(svcs))
			r.Post("/image:ingest", ingestCharacterImage(svcs))
		})
		r.Route("/shots/{id}", func(r chi.Router) {
			r.Get("/", getShot(svcs))
			r.Post("/image:regen", regenShotImage(svcs))
			r.Post("/tts:regen", regenShotTTS(svcs))
			r.Post("/assets:ingest", ingestShotAssets(svcs))
		})
		r.Get("/jobs/{id}", getJob(svcs))
		r.Get("/assets/{id}", getAsset(svcs))
		r.Get("/ws/projects/{id}", wsProject(svcs))
	})
	return r
}
