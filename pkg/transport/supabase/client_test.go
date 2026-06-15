// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: FSL-1.1-ALv2

//go:build unit

package supabase

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		status int
		want   resource.OperationErrorCode
	}{
		{400, resource.OperationErrorCodeInvalidRequest},
		{401, resource.OperationErrorCodeInvalidCredentials},
		{403, resource.OperationErrorCodeAccessDenied},
		{404, resource.OperationErrorCodeNotFound},
		{409, resource.OperationErrorCodeAlreadyExists},
		{422, resource.OperationErrorCodeInvalidRequest},
		{429, resource.OperationErrorCodeThrottling},
		{500, resource.OperationErrorCodeServiceInternalError},
		{503, resource.OperationErrorCodeServiceInternalError},
		{418, resource.OperationErrorCodeInternalFailure},
	}
	for _, tc := range cases {
		if got := ClassifyStatus(tc.status); got != tc.want {
			t.Errorf("ClassifyStatus(%d) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestNewClientRequiresToken(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("expected error when AccessToken empty")
	}
}

func TestDo_Success(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/projects/abc" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"abc","name":"hello"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{BaseURL: srv.URL, AccessToken: "tok"})
	var out struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := c.Do(context.Background(), Request{Method: "GET", Path: "/v1/projects/abc"}, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.ID != "abc" || out.Name != "hello" {
		t.Fatalf("unexpected payload %+v", out)
	}
	if capturedAuth != "Bearer tok" {
		t.Errorf("Authorization = %q", capturedAuth)
	}
}

func TestDo_Body(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var got map[string]string
		_ = json.Unmarshal(b, &got)
		if got["name"] != "demo" {
			t.Errorf("body name = %v", got)
		}
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"id":"new"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{BaseURL: srv.URL, AccessToken: "tok"})
	var out struct{ ID string }
	err := c.Do(context.Background(), Request{
		Method: "POST", Path: "/v1/projects",
		Body: map[string]string{"name": "demo"},
	}, &out)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.ID != "new" {
		t.Fatalf("ID = %q", out.ID)
	}
}

func TestDo_ErrorReturnsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = io.WriteString(w, `{"message":"not found"}`)
	}))
	defer srv.Close()

	c, _ := NewClient(Config{BaseURL: srv.URL, AccessToken: "tok"})
	err := c.Do(context.Background(), Request{Method: "GET", Path: "/v1/projects/missing"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound false for %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("message missing in %v", err)
	}
}
