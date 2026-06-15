// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: FSL-1.1-ALv2

//go:build unit

package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

func clientFor(t *testing.T, h http.HandlerFunc) (*supatransport.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	c, _ := supatransport.NewClient(supatransport.Config{BaseURL: srv.URL, AccessToken: "tok"})
	return c, srv
}

func TestAPIKey_Create_AddsRevealQuery(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("reveal") != "true" {
			t.Errorf("reveal missing")
		}
		_, _ = io.WriteString(w, `{"id":"k1","api_key":"sb_secret_x","type":"secret","name":"ci_key"}`)
	})
	defer srv.Close()
	a := &APIKey{Client: c}
	props, _ := json.Marshal(APIKeyProperties{ProjectRef: "p1", Name: "ci_key", Type: "secret"})
	res, _ := a.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.NativeID != "p1/k1" {
		t.Fatalf("NativeID = %q", res.ProgressResult.NativeID)
	}
}

func TestAPIKey_Update_Patches(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %s", r.Method)
		}
		_, _ = io.WriteString(w, `{"id":"k1","name":"new_name"}`)
	})
	defer srv.Close()
	a := &APIKey{Client: c}
	props, _ := json.Marshal(APIKeyProperties{Name: "new_name"})
	res, _ := a.Update(context.Background(), &resource.UpdateRequest{NativeID: "p1/k1", DesiredProperties: props})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestAPIKey_Delete_Idempotent(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	defer srv.Close()
	a := &APIKey{Client: c}
	res, _ := a.Delete(context.Background(), &resource.DeleteRequest{NativeID: "p1/k1"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}
