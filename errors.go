// Package prismgoclient defines Prism credentials, filters and HTTP diagnostics.
package prismgoclient

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrResponseTooLarge indicates that an API response exceeds the SDK byte budget.
var ErrResponseTooLarge = errors.New("response body exceeds SDK limit")

// ResponseError identifies an HTTP or response-body failure without echoing server data.
// Inspect Cause with errors.Is/As rather than logging its potentially untrusted text.
type ResponseError struct {
	StatusCode int
	Cause      error
}

func (e *ResponseError) Error() string {
	if e.StatusCode == http.StatusUnauthorized {
		return "invalid Nutanix credentials"
	}

	if e.StatusCode >= http.StatusOK && e.StatusCode < http.StatusMultipleChoices {
		return fmt.Sprintf("invalid HTTP response body (status %d)", e.StatusCode)
	}

	return fmt.Sprintf("unexpected HTTP status %d (%s)", e.StatusCode, http.StatusText(e.StatusCode))
}

func (e *ResponseError) Unwrap() error { return e.Cause }
