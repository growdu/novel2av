package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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

// --- Chapters ---------------------------------------------------------------

func listChapters(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		items, err := svcs.Chapter.List(r.Context(), projectID)
		if mapErr(w, err) {
			return
		}
		if items == nil {
			items = []domain.Chapter{}
		}
		jsonResp(w, http.StatusOK, items)
	}
}

func splitChapters(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		jobID, err := svcs.Chapter.TriggerSplit(r.Context(), projectID)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
	}
}

func ingestChapters(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		n, err := svcs.Chapter.IngestSplitResult(r.Context(), projectID)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusOK, map[string]any{"ingested": n})
	}
}

func patchChapter(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body struct {
			Title  *string `json:"title"`
			Status *string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		c, err := svcs.Chapter.Patch(r.Context(), id, body.Title, body.Status)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusOK, c)
	}
}

// --- Other placeholders -----------------------------------------------------

func runPipeline(svcs *service.Services) http.HandlerFunc      { return notImpl }
func rerunPipeline(svcs *service.Services) http.HandlerFunc    { return notImpl }
// --- Characters -------------------------------------------------------------

func listCharacters(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		items, err := svcs.Character.List(r.Context(), projectID)
		if mapErr(w, err) {
			return
		}
		if items == nil {
			items = []domain.Character{}
		}
		// Decorate with ref_image_url (signed).
		ttl := 15 * time.Minute
		for i := range items {
			if items[i].RefImageKey == "" {
				continue
			}
			u, err := svcs.Asset.URL(r.Context(), items[i].RefImageKey, ttl)
			if err == nil {
				items[i].Meta = map[string]any{"ref_image_url": u.String()}
			}
		}
		jsonResp(w, http.StatusOK, items)
	}
}

func extractCharacters(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		jobID, err := svcs.Character.TriggerExtract(r.Context(), projectID)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
	}
}

func ingestCharacters(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		n, err := svcs.Character.IngestExtractResult(r.Context(), projectID)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusOK, map[string]any{"ingested": n})
	}
}

func getCharacter(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		c, err := svcs.Character.Get(r.Context(), id)
		if mapErr(w, err) {
			return
		}
		if c.RefImageKey != "" {
			if u, err := svcs.Asset.URL(r.Context(), c.RefImageKey, 30*time.Minute); err == nil {
				c.Meta = map[string]any{"ref_image_url": u.String()}
			}
		}
		jsonResp(w, http.StatusOK, c)
	}
}

func patchCharacter(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body domain.CharacterPatch
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		c, err := svcs.Character.Patch(r.Context(), id, body)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusOK, c)
	}
}

func regenCharacterImage(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body struct {
			Variants int `json:"variants"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		jobID, err := svcs.Character.TriggerRegenImage(r.Context(), id, body.Variants)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
	}
}

func ingestCharacterImage(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body struct {
			RefImageKey string `json:"ref_image_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		if body.RefImageKey == "" {
			errResp(w, http.StatusBadRequest, "invalid_input", "ref_image_key required")
			return
		}
		if mapErr(w, svcs.Character.IngestCharacterImage(r.Context(), id, body.RefImageKey)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteCharacter(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if mapErr(w, svcs.Character.Delete(r.Context(), id)) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- placeholders for the remaining surfaces ------------------------------

func listShots(svcs *service.Services) http.HandlerFunc           { return notImpl }
func regenShotImage(svcs *service.Services) http.HandlerFunc      { return notImpl }
func regenShotTTS(svcs *service.Services) http.HandlerFunc        { return notImpl }
func composeChapterVideo(svcs *service.Services) http.HandlerFunc { return notImpl }
func getJob(svcs *service.Services) http.HandlerFunc               { return notImpl }
func getAsset(svcs *service.Services) http.HandlerFunc             { return notImpl }
func wsProject(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		errResp(w, http.StatusNotImplemented, "not_implemented", "websocket handler pending")
	}
}

func notImpl(w http.ResponseWriter, _ *http.Request) {
	errResp(w, http.StatusNotImplemented, "not_implemented", "handler pending")
}

func limitOr(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
