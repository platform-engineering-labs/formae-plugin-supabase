// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

//go:build unit

package config

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

func TestAuthSettings_PatchesConfigAuth(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/v1/projects/p1/config/auth" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"site_url":"https://x"}`)
	})
	defer srv.Close()
	a := &AuthSettings{singleton: singleton{client: c, pathSuffix: "/config/auth", writeMethod: "PATCH"}}
	props, _ := json.Marshal(Properties{ProjectRef: "p1", Settings: map[string]any{"site_url": "https://x"}})
	res, _ := a.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status %v", res.ProgressResult.OperationStatus)
	}
}

func TestDatabaseSettings_UsesPUT(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/v1/projects/p1/config/database/postgres" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"max_connections":200}`)
	})
	defer srv.Close()
	d := &DatabaseSettings{singleton: singleton{client: c, pathSuffix: "/config/database/postgres", writeMethod: "PUT"}}
	props, _ := json.Marshal(Properties{ProjectRef: "p1", Settings: map[string]any{"max_connections": 200}})
	res, _ := d.Update(context.Background(), &resource.UpdateRequest{NativeID: "p1", DesiredProperties: props})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status %v", res.ProgressResult.OperationStatus)
	}
}

func TestAPISettings_ReadHitsPostgrest(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/p1/postgrest" {
			t.Errorf("path %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"db_schema":"public"}`)
	})
	defer srv.Close()
	a := &APISettings{singleton: singleton{client: c, pathSuffix: "/postgrest", writeMethod: "PATCH"}}
	res, _ := a.Read(context.Background(), &resource.ReadRequest{NativeID: "p1"})
	if res.ErrorCode != "" {
		t.Fatalf("ErrorCode %v", res.ErrorCode)
	}
}

func TestSingleton_DeleteIsNoop(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not call API") })
	defer srv.Close()
	n := &NetworkRestriction{singleton: singleton{client: c, pathSuffix: "/network-restrictions", writeMethod: "PATCH"}}
	res, _ := n.Delete(context.Background(), &resource.DeleteRequest{NativeID: "p1"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status %v", res.ProgressResult.OperationStatus)
	}
}
