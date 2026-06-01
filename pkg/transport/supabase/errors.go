// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

package supabase

import (
	"errors"
	"fmt"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

// APIError represents a non-2xx response from the Supabase Management API.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("supabase API: HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("supabase API: HTTP %d: %s", e.StatusCode, e.Body)
}

// IsNotFound reports whether err represents a missing resource.
//
// Most Supabase Management API endpoints return 404 for unknown ids, but some
// (notably DELETE/GET on `/api-keys/{id}` for a deleted key) return 406 with
// a body like `{"message":"Failed to find API key for project"}`. Treat that
// shape as NotFound too so sync after out-of-band deletes converges.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.StatusCode == 404 {
		return true
	}
	if apiErr.StatusCode == 406 && containsAny(apiErr.Message,
		"Failed to find", "not found", "does not exist") {
		return true
	}
	// Supabase's project API returns 400 'Resource has been removed' for
	// GET/PATCH/DELETE after a project's been deleted (rather than 404).
	if apiErr.StatusCode == 400 && containsAny(apiErr.Message,
		"has been removed", "Resource has been removed", "is being removed") {
		return true
	}
	return false
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if n != "" && indexFold(s, n) >= 0 {
			return true
		}
	}
	return false
}

// indexFold reports the first case-insensitive index of substr in s, or -1.
func indexFold(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// ClassifyStatus maps an HTTP status to a formae operation error code.
func ClassifyStatus(status int) resource.OperationErrorCode {
	switch {
	case status == 400, status == 422:
		return resource.OperationErrorCodeInvalidRequest
	case status == 401:
		return resource.OperationErrorCodeInvalidCredentials
	case status == 403:
		return resource.OperationErrorCodeAccessDenied
	case status == 404:
		return resource.OperationErrorCodeNotFound
	case status == 409:
		return resource.OperationErrorCodeAlreadyExists
	case status == 429:
		return resource.OperationErrorCodeThrottling
	case status >= 500 && status <= 599:
		return resource.OperationErrorCodeServiceInternalError
	default:
		return resource.OperationErrorCodeInternalFailure
	}
}

// ClassifyError maps a Go error from Do() to a formae operation error code.
func ClassifyError(err error) resource.OperationErrorCode {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return ClassifyStatus(apiErr.StatusCode)
	}
	return resource.OperationErrorCodeInternalFailure
}
