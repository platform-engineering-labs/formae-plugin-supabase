// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

//go:build unit

package supabase

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain string error", errors.New("boom"), false},
		{"404", &APIError{StatusCode: 404, Message: "anything"}, true},
		{
			"406 with Failed to find",
			&APIError{StatusCode: 406, Message: "Failed to find API key for project"},
			true,
		},
		{
			"406 with not found",
			&APIError{StatusCode: 406, Message: "Item not found"},
			true,
		},
		{
			"406 with does not exist",
			&APIError{StatusCode: 406, Message: "key does not exist"},
			true,
		},
		{
			"406 unrelated message",
			&APIError{StatusCode: 406, Message: "rate limited"},
			false,
		},
		{
			"400 'Resource has been removed' is NOT promoted at transport layer",
			&APIError{StatusCode: 400, Message: "Resource has been removed"},
			false,
		},
		{
			"403 'necessary privileges' is NOT promoted at transport layer",
			&APIError{StatusCode: 403, Message: "Your account does not have the necessary privileges"},
			false,
		},
		{
			"500 not NotFound",
			&APIError{StatusCode: 500, Message: "internal"},
			false,
		},
		{
			"wrapped 404",
			fmt.Errorf("wrapped: %w", &APIError{StatusCode: 404}),
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

