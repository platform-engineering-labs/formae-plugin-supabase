// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

//go:build unit

package platform

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
	c, err := supatransport.NewClient(supatransport.Config{BaseURL: srv.URL, AccessToken: "tok"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

func TestProject_Create_AsyncInProgress(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/projects" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"id":"abcdef","status":"COMING_UP"}`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	props, _ := json.Marshal(ProjectProperties{
		Name: "demo", OrganizationID: "org_1", Region: "us-east-1", DBPass: "longpass1",
	})
	res, _ := p.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.OperationStatus != resource.OperationStatusInProgress {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	if res.ProgressResult.NativeID != "abcdef" || res.ProgressResult.RequestID != "abcdef" {
		t.Fatalf("ids = %+v", res.ProgressResult)
	}
}

func TestProject_Create_RejectsMissingFields(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("API must not be called") })
	defer srv.Close()
	p := &Project{Client: c}
	props, _ := json.Marshal(ProjectProperties{Name: "x"})
	res, _ := p.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.ErrorCode != resource.OperationErrorCodeInvalidRequest {
		t.Fatalf("ErrorCode = %v", res.ProgressResult.ErrorCode)
	}
}

func TestProject_Status_Healthy(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"abc","status":"ACTIVE_HEALTHY"}`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Status(context.Background(), &resource.StatusRequest{RequestID: "abc"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestProject_Status_InProgress(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"abc","status":"COMING_UP"}`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Status(context.Background(), &resource.StatusRequest{RequestID: "abc"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusInProgress {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestProject_Read_NotFound(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Read(context.Background(), &resource.ReadRequest{NativeID: "abc"})
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode = %v", res.ErrorCode)
	}
}

func TestProject_Delete_Idempotent(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Delete(context.Background(), &resource.DeleteRequest{NativeID: "abc"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestProject_List(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"id":"a"},{"id":"b"},{"id":"c"}]`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.List(context.Background(), &resource.ListRequest{})
	if len(res.NativeIDs) != 3 {
		t.Fatalf("ids = %v", res.NativeIDs)
	}
}
