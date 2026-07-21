package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/novel2av/backend/internal/service"
)

// json is a tiny helper for typed responses.
func json[T any](w http.ResponseWriter, status int, v T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, code, msg string) {
	json(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func readyz(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svcs.Ping(r.Context()); err != nil {
			errJSON(w, http.StatusServiceUnavailable, "not_ready", err.Error())
			return
		}
		json(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func createProject(svcs *service.Services) http.HandlerFunc { return notImpl }
func listProjects(svcs *service.Services) http.HandlerFunc   { return notImpl }
func getProject(svcs *service.Services) http.HandlerFunc     { return notImpl }
func deleteProject(svcs *service.Services) http.HandlerFunc  { return notImpl }
func runPipeline(svcs *service.Services) http.HandlerFunc    { return notImpl }
func rerunPipeline(svcs *service.Services) http.HandlerFunc  { return notImpl }
func listChapters(svcs *service.Services) http.HandlerFunc   { return notImpl }
func splitChapters(svcs *service.Services) http.HandlerFunc  { return notImpl }
func patchChapter(svcs *service.Services) http.HandlerFunc   { return notImpl }
func listCharacters(svcs *service.Services) http.HandlerFunc { return notImpl }
func extractCharacters(svcs *service.Services) http.HandlerFunc {
	return notImpl
}
func regenCharacterImage(svcs *service.Services) http.HandlerFunc { return notImpl }
func listShots(svcs *service.Services) http.HandlerFunc           { return notImpl }
func regenShotImage(svcs *service.Services) http.HandlerFunc      { return notImpl }
func regenShotTTS(svcs *service.Services) http.HandlerFunc        { return notImpl }
func composeChapterVideo(svcs *service.Services) http.HandlerFunc { return notImpl }
func getJob(svcs *service.Services) http.HandlerFunc               { return notImpl }
func getAsset(svcs *service.Services) http.HandlerFunc             { return notImpl }
func wsProject(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		_ = projectID
		errJSON(w, http.StatusNotImplemented, "not_implemented", "websocket handler pending")
	}
}

func notImpl(w http.ResponseWriter, _ *http.Request) {
	errJSON(w, http.StatusNotImplemented, "not_implemented", "handler pending")
}
