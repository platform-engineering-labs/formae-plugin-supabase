// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

//go:build unit

package database

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

func TestPoolerConfig_Create_PatchesPooler(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/v1/projects/p1/config/database/pooler" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var got map[string]any
		_ = json.Unmarshal(b, &got)
		if got["pool_mode"] != "transaction" || got["default_pool_size"] != float64(25) {
			t.Errorf("body wrong: %v", got)
		}
		_, _ = io.WriteString(w, `{"pool_mode":"transaction","default_pool_size":25}`)
	})
	defer srv.Close()

	pc := &PoolerConfig{client: c}
	props, _ := json.Marshal(Properties{
		ProjectRef: "p1",
		Settings: map[string]any{
			"pool_mode":         "transaction",
			"default_pool_size": 25,
		},
	})
	res, _ := pc.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.NativeID != "p1" || res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("create = %+v", res.ProgressResult)
	}
}

func TestPoolerConfig_PatchSendsOnlyMutableFields(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var got map[string]any
		_ = json.Unmarshal(b, &got)
		if _, ok := got["db_host"]; ok {
			t.Errorf("db_host (read-only) leaked into PATCH body: %v", got)
		}
		_, _ = io.WriteString(w, `{}`)
	})
	defer srv.Close()
	pc := &PoolerConfig{client: c}
	props, _ := json.Marshal(Properties{
		ProjectRef: "p1",
		Settings: map[string]any{
			"pool_mode": "session",
			"db_host":   "should-not-be-patched.example",
		},
	})
	_, _ = pc.Update(context.Background(), &resource.UpdateRequest{NativeID: "p1", DesiredProperties: props})
}

func TestPoolerConfig_Read_UnwrapsArrayPicksPrimary(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/projects/p1/config/database/pooler" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `[
		    {"identifier":"replica-1","database_type":"READ_REPLICA","pool_mode":"transaction","db_user":"replica"},
		    {"identifier":"primary","database_type":"PRIMARY","pool_mode":"transaction","db_user":"primary","default_pool_size":25}
		]`)
	})
	defer srv.Close()
	pc := &PoolerConfig{client: c}
	res, _ := pc.Read(context.Background(), &resource.ReadRequest{NativeID: "p1"})
	if res.ErrorCode != "" {
		t.Fatalf("ErrorCode = %v", res.ErrorCode)
	}
	var got Properties
	_ = json.Unmarshal([]byte(res.Properties), &got)
	if got.Settings["database_type"] != "PRIMARY" || got.Settings["db_user"] != "primary" {
		t.Fatalf("expected primary entry, got %+v", got.Settings)
	}
}

func TestPoolerConfig_Read_FallsBackToFirstEntry(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"identifier":"only","db_user":"x","pool_mode":"transaction"}]`)
	})
	defer srv.Close()
	pc := &PoolerConfig{client: c}
	res, _ := pc.Read(context.Background(), &resource.ReadRequest{NativeID: "p1"})
	if res.ErrorCode != "" {
		t.Fatalf("ErrorCode = %v", res.ErrorCode)
	}
}

func TestPoolerConfig_Read_404(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	defer srv.Close()
	pc := &PoolerConfig{client: c}
	res, _ := pc.Read(context.Background(), &resource.ReadRequest{NativeID: "p1"})
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode = %v", res.ErrorCode)
	}
}

func TestPoolerConfig_Delete_NoOp(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("API must not be called") })
	defer srv.Close()
	pc := &PoolerConfig{client: c}
	res, _ := pc.Delete(context.Background(), &resource.DeleteRequest{NativeID: "p1"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestPoolerConfig_List_FromProjects(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"id":"a"},{"id":"b"}]`)
	})
	defer srv.Close()
	pc := &PoolerConfig{client: c}
	res, _ := pc.List(context.Background(), &resource.ListRequest{})
	if len(res.NativeIDs) != 2 {
		t.Fatalf("ids = %v", res.NativeIDs)
	}
}
