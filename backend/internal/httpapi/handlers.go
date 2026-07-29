package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/novel2av/backend/internal/domain"
	"github.com/novel2av/backend/internal/infra/db/repo"
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
				items[i].Meta = map[string]any{"ref_image_url": u}
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
				c.Meta = map[string]any{"ref_image_url": u}
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

// --- Project video (full book) --------------------------------------------

func getProjectVideo(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		v, err := svcs.ProjectVideo.Get(r.Context(), projectID)
		if mapErr(w, err) { return }
		if v.VideoKey != "" {
			if u, err := svcs.Asset.URL(r.Context(), v.VideoKey, 60*time.Minute); err == nil {
				jsonResp(w, http.StatusOK, map[string]any{
					"project_id": v.ProjectID, "video_url": u,
					"duration_sec": v.DurationSec, "status": v.Status, "error": v.Error,
				})
				return
			}
		}
		jsonResp(w, http.StatusOK, v)
	}
}

func composeProjectVideo(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		jobID, err := svcs.ProjectVideo.TriggerCompose(r.Context(), projectID)
		if mapErr(w, err) { return }
		jsonResp(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
	}
}

func ingestProjectVideo(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		var body struct {
			VideoKey    string  `json:"video_key"`
			DurationSec float64 `json:"duration_sec"`
			Status      string  `json:"status"`
			Error       string  `json:"error"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", err.Error()); return
		}
		if body.Status == "" { body.Status = "READY" }
		if err := svcs.ProjectVideo.IngestComposeResult(r.Context(), projectID, body.VideoKey, body.DurationSec, body.Status, body.Error); err != nil {
			mapErr(w, err); return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- placeholders for the remaining surfaces ------------------------------

func getShot(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		sh, err := svcs.Shot.Get(r.Context(), id)
		if mapErr(w, err) { return }
		ttl := 30 * time.Minute
		if sh.ImageKey != "" {
			if u, err := svcs.Asset.URL(r.Context(), sh.ImageKey, ttl); err == nil {
				if sh.Meta == nil { sh.Meta = map[string]any{} }
				sh.Meta["image_url"] = u
			}
		}
		if sh.TTSKey != "" {
			if u, err := svcs.Asset.URL(r.Context(), sh.TTSKey, ttl); err == nil {
				if sh.Meta == nil { sh.Meta = map[string]any{} }
				sh.Meta["tts_url"] = u
			}
		}
		if sh.BGMKey != "" {
			if u, err := svcs.Asset.URL(r.Context(), sh.BGMKey, ttl); err == nil {
				if sh.Meta == nil { sh.Meta = map[string]any{} }
				sh.Meta["bgm_url"] = u
			}
		}
		jsonResp(w, http.StatusOK, sh)
	}
}

// --- Shots ------------------------------------------------------------------

func listShots(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		items, err := svcs.Shot.ListByProject(r.Context(), projectID)
		if mapErr(w, err) { return }
		if items == nil {
			items = []domain.Shot{}
		}
		// Decorate with signed URLs for any media we have.
		ttl := 30 * time.Minute
		for i := range items {
			if items[i].ImageKey != "" {
				if u, err := svcs.Asset.URL(r.Context(), items[i].ImageKey, ttl); err == nil {
					if items[i].Meta == nil { items[i].Meta = map[string]any{} }
					items[i].Meta["image_url"] = u
				}
			}
			if items[i].TTSKey != "" {
				if u, err := svcs.Asset.URL(r.Context(), items[i].TTSKey, ttl); err == nil {
					if items[i].Meta == nil { items[i].Meta = map[string]any{} }
					items[i].Meta["tts_url"] = u
				}
			}
			if items[i].BGMKey != "" {
				if u, err := svcs.Asset.URL(r.Context(), items[i].BGMKey, ttl); err == nil {
					if items[i].Meta == nil { items[i].Meta = map[string]any{} }
					items[i].Meta["bgm_url"] = u
				}
			}
		}
		jsonResp(w, http.StatusOK, items)
	}
}

func triggerBreakdown(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		jobIDs, err := svcs.Shot.TriggerProjectBreakdown(r.Context(), projectID)
		if mapErr(w, err) { return }
		jsonResp(w, http.StatusAccepted, map[string]any{"job_ids": jobIDs, "status": "queued"})
	}
}

func ingestBreakdown(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		var body struct {
			ChapterID string `json:"chapter_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ChapterID == "" {
			errResp(w, http.StatusBadRequest, "invalid_input", "chapter_id required")
			return
		}
		n, err := svcs.Shot.IngestBreakdown(r.Context(), projectID, body.ChapterID)
		if mapErr(w, err) { return }
		jsonResp(w, http.StatusOK, map[string]any{"ingested": n})
	}
}

func regenShotImage(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body struct{ Aspect string `json:"aspect"` }
		_ = json.NewDecoder(r.Body).Decode(&body)
		jobID, err := svcs.Shot.TriggerGenerateShot(r.Context(), id, body.Aspect)
		if mapErr(w, err) { return }
		jsonResp(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
	}
}

func regenShotTTS(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		jobID, err := svcs.Shot.TriggerGenerateShot(r.Context(), id, "")
		if mapErr(w, err) { return }
		jsonResp(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
	}
}

func ingestShotAssets(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body struct {
			ImageKey    *string `json:"image_key,omitempty"`
			TTSKey      *string `json:"tts_key,omitempty"`
			BGMKey      *string `json:"bgm_key,omitempty"`
			SubtitleKey *string `json:"subtitle_key,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", err.Error()); return
		}
		sh, err := svcs.Shot.IngestShotAssets(r.Context(), id, service.ShotAssetPatch{
			ImageKey: body.ImageKey, TTSKey: body.TTSKey, BGMKey: body.BGMKey, SubtitleKey: body.SubtitleKey,
		})
		if mapErr(w, err) { return }
		jsonResp(w, http.StatusOK, sh)
	}
}

func composeChapterVideo(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chapterID := chi.URLParam(r, "id")
		var body struct {
			Aspect string `json:"aspect"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		jobID, err := svcs.Composition.TriggerCompose(r.Context(), chapterID, body.Aspect)
		if mapErr(w, err) { return }
		jsonResp(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
	}
}

func composeAllChapters(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		var body struct{ Aspect string `json:"aspect"` }
		_ = json.NewDecoder(r.Body).Decode(&body)
		jobIDs, err := svcs.Composition.TriggerProjectCompose(r.Context(), projectID, body.Aspect)
		if mapErr(w, err) { return }
		jsonResp(w, http.StatusAccepted, map[string]any{"job_ids": jobIDs, "status": "queued"})
	}
}

func getChapterVideo(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chapterID := chi.URLParam(r, "id")
		v, err := svcs.Composition.Get(r.Context(), chapterID)
		if mapErr(w, err) { return }
		if v.VideoKey != "" {
			if u, err := svcs.Asset.URL(r.Context(), v.VideoKey, 60*time.Minute); err == nil {
				jsonResp(w, http.StatusOK, map[string]any{
					"chapter_id": v.ChapterID, "video_url": u,
					"duration_sec": v.DurationSec, "status": v.Status, "error": v.Error,
				})
				return
			}
		}
		jsonResp(w, http.StatusOK, v)
	}
}

func listChapterVideos(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		items, err := svcs.Composition.ListByProject(r.Context(), projectID)
		if mapErr(w, err) { return }
		if items == nil { items = []repo.ChapterVideo{} }
		// decorate with video_url
		for i := range items {
			if items[i].VideoKey == "" { continue }
			if u, err := svcs.Asset.URL(r.Context(), items[i].VideoKey, 60*time.Minute); err == nil {
				// stash into a sibling field via an inline struct
				_ = u
			}
		}
		jsonResp(w, http.StatusOK, items)
	}
}

func ingestChapterVideo(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chapterID := chi.URLParam(r, "id")
		var body struct {
			VideoKey    string  `json:"video_key"`
			DurationSec float64 `json:"duration_sec"`
			Status      string  `json:"status"`
			Error       string  `json:"error"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", err.Error()); return
		}
		if body.Status == "" { body.Status = "READY" }
		if err := svcs.Composition.IngestComposeResult(r.Context(), chapterID, body.VideoKey, body.DurationSec, body.Status, body.Error); err != nil {
			mapErr(w, err); return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
func getJob(svcs *service.Services) http.HandlerFunc               { return notImpl }
func getAsset(svcs *service.Services) http.HandlerFunc             { return notImpl }
func wsProject(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		svcs.Events.ServeWS(w, r, projectID)
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
