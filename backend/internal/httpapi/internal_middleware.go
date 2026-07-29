package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// defaultInternalSecret is the placeholder the ai-engine uses in dev. The
// production backend rejects any HMAC secret that equals this so a forgotten
// env var (or copy-pasted dev secret) cannot ship by accident.
const defaultInternalSecret = "dev-secret"

// EnsureInternalSecurity rejects startup when running outside "dev" and the
// AI_BACKEND_HMAC_SECRET is missing or still set to the default. Returns a
// descriptive error so the caller can decide whether to log + exit.
func EnsureInternalSecurity(appEnv string) error {
	if appEnv == "dev" {
		return nil
	}
	secret := os.Getenv("AI_BACKEND_HMAC_SECRET")
	if secret == "" || secret == defaultInternalSecret {
		return fmt.Errorf("AI_BACKEND_HMAC_SECRET must be set to a non-default value when APP_ENV=%q", appEnv)
	}
	return nil
}

// verifyInternalSignature checks the X-N2AV-Signature header against
// HMAC-SHA256(body, secret). The secret defaults to "dev-secret" if the
// operator has not set AI_BACKEND_HMAC_SECRET. Empty header always fails.
func verifyInternalSignature(header string, body []byte) bool {
	if header == "" {
		return false
	}
	secret := os.Getenv("AI_BACKEND_HMAC_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(header), []byte(want))
}
