package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrRespShape(t *testing.T) {
	rr := httptest.NewRecorder()
	errResp(rr, http.StatusBadRequest, "invalid_input", "bad")
	require.Equal(t, http.StatusBadRequest, rr.Code)
	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "invalid_input", body["error"]["code"])
	require.Equal(t, "bad", body["error"]["message"])
}

func TestLimitOr(t *testing.T) {
	require.Equal(t, 20, limitOr(0, 20))
	require.Equal(t, 7, limitOr(7, 20))
	require.Equal(t, 20, limitOr(-1, 20))
}
