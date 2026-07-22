package service

import (
	"context"
	"io"
	"net/http"
	"time"
)

// fetchURL does a tiny GET used by ingestion helpers. Centralised so tests
// can stub it later if needed.
func fetchURL(ctx context.Context, url string) ([]byte, error) {
	c := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmtErr(resp.Status)
	}
	return io.ReadAll(resp.Body)
}

type statusErr struct{ code int }

func (e *statusErr) Error() string { return "non-2xx response: " + itoa(e.code) }
func fmtErr(c int) error           { return &statusErr{code: c} }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(b[pos:])
}
