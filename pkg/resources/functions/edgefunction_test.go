// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: FSL-1.1-ALv2

//go:build unit

package functions

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

func TestEdgeFunction_Create(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/p1/functions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"f1","slug":"hello","name":"Hello","version":1}`)
	})
	defer srv.Close()
	e := &EdgeFunction{Client: c}
	props, _ := json.Marshal(EdgeFunctionProperties{
		ProjectRef: "p1", Slug: "hello", Name: "Hello",
		Body: "Deno.serve(()=>new Response('hi'))",
	})
	res, _ := e.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.NativeID != "p1/hello" {
		t.Fatalf("NativeID = %q", res.ProgressResult.NativeID)
	}
}

func TestEdgeFunction_Read(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"f1","slug":"hello","name":"Hello","status":"ACTIVE","version":3}`)
	})
	defer srv.Close()
	e := &EdgeFunction{Client: c}
	res, _ := e.Read(context.Background(), &resource.ReadRequest{NativeID: "p1/hello"})
	if res.ErrorCode != "" {
		t.Fatalf("ErrorCode %v", res.ErrorCode)
	}
	var got EdgeFunctionProperties
	_ = json.Unmarshal([]byte(res.Properties), &got)
	if got.Status != "ACTIVE" || got.Version != 3 {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestEdgeFunction_Delete_Idempotent(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	defer srv.Close()
	e := &EdgeFunction{Client: c}
	res, _ := e.Delete(context.Background(), &resource.DeleteRequest{NativeID: "p1/hello"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}
