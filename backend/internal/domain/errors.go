package domain

import "errors"

// Sentinel errors used across the service layer; transport maps them to HTTP.
var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUpstreamFailure = errors.New("upstream failure")
)
