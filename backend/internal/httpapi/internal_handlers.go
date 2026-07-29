package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/novel2av/backend/internal/service"
)

// internalJobComplete is the callback from the ai-engine when a Celery task
// finishes. The signature in `X-N2AV-Signature` is HMAC-SHA256(body, secret).
// Only same-network callers (the ai-engine worker) can reach this path; the
// shared secret is the second line of defense.
func internalJobComplete(svcs *service.Services) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskID := chi.URLParam(r, "id")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", "read body: "+err.Error())
			return
		}
		if !verifyInternalSignature(r.Header.Get("X-N2AV-Signature"), body) {
			errResp(w, http.StatusUnauthorized, "unauthorized", "bad signature")
			return
		}
		var msg struct {
			Task      string          `json:"task"`
			ProjectID string          `json:"project_id"`
			Payload   json.RawMessage `json:"payload"`
			Result    json.RawMessage `json:"result"`
			Error     string          `json:"error,omitempty"`
		}
		if err := json.Unmarshal(body, &msg); err != nil {
			errResp(w, http.StatusBadRequest, "invalid_input", "decode: "+err.Error())
			return
		}
		out, err := svcs.IngestTaskResult(r.Context(), msg.Task, taskID, msg.ProjectID, msg.Payload, msg.Result, msg.Error)
		if mapErr(w, err) {
			return
		}
		jsonResp(w, http.StatusOK, map[string]any{"status": "ok", "result": out})
	}
}
