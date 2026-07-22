package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/novel2av/backend/internal/domain"
	"github.com/novel2av/backend/internal/service"
)

func jsonResp[T any](w http.ResponseWriter, status int, v T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errResp(w http.ResponseWriter, status int, code, msg string) {
	jsonResp(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

// mapErr maps domain sentinels to HTTP responses.
func mapErr(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		errResp(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrConflict):
		errResp(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		errResp(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, domain.ErrUpstreamFailure):
		errResp(w, http.StatusBadGateway, "upstream_error", err.Error())
	default:
		errResp(w, http.StatusInternalServerError, "internal", err.Error())
	}
	return true
}

// currentUser returns a stub user_id for M1. Replace with JWT/session in M7+.
func currentUser(_ *http.Request) string { return "00000000-0000-0000-0000-000000000001" }

func readyz(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svcs.Ping(r.Context()); err != nil {
			errResp(w, http.StatusServiceUnavailable, "not_ready", err.Error())
			return
		}
		jsonResp(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

// --- Projects ---------------------------------------------------------------

func createProject(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 32 MB in-memory cap for the multipart body.
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", "multipart parse: "+err.Error())
			return
		}
		title := r.FormValue("title")
		author := r.FormValue("author")
		cfgRaw := r.FormValue("config")
		file, header, err := r.FormFile("file")
		if err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", "file required")
			return
		}
		defer file.Close()

		var cfg domain.ProjectConfig
		if cfgRaw != "" {
			if err := json.Unmarshal([]byte(cfgRaw), &cfg); err != nil {
				errResp(w, http.StatusBadRequest, "invalid_input", "config json: "+err.Error())
				return
			}
		}

		created, err := svcs.Project.Create(r.Context(), service.CreateProjectInput{
			UserID:   currentUser(r),
			Title:    title,
			Author:   author,
			Filename: header.Filename,
			Content:  file,
			Size:     header.Size,
			Config:   cfg,
		})
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusCreated, created)
	}
}

func listProjects(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		items, err := svcs.Project.List(r.Context(), currentUser(r), limit, offset)
		if mapErr(w, err) {
			return
		}
		if items == nil {
			items = []domain.Project{}
		}
		jsonResp(w, http.StatusOK, map[string]any{"items": items, "limit": limitOr(limit, 20), "offset": offset})
	}
}

func getProject(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		p, err := svcs.Project.Get(r.Context(), id)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusOK, p)
	}
}

func deleteProject(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if mapErr(w, svcs.Project.Delete(r.Context(), id)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func limitOr(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// --- stubs kept so the router still compiles for non-M1 endpoints -----------

func runPipeline(svcs *service.Services) http.HandlerFunc      { return notImpl }
func rerunPipeline(svcs *service.Services) http.HandlerFunc    { return notImpl }
func listChapters(svcs *service.Services) http.HandlerFunc     { return notImpl }
func splitChapters(svcs *service.Services) http.HandlerFunc    { return notImpl }
func patchChapter(svcs *service.Services) http.HandlerFunc     { return notImpl }
func listCharacters(svcs *service.Services) http.HandlerFunc   { return notImpl }
func extractCharacters(svcs *service.Services) http.HandlerFunc { return notImpl }
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
		errResp(w, http.StatusNotImplemented, "not_implemented", "websocket handler pending")
	}
}

func notImpl(w http.ResponseWriter, _ *http.Request) {
	errResp(w, http.StatusNotImplemented, "not_implemented", "handler pending")
}
