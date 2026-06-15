// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: FSL-1.1-ALv2

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

func TestProject_List_FiltersRemoved(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"id":"keep1","status":"ACTIVE_HEALTHY"},{"id":"gone","status":"REMOVED"},{"id":"keep2","status":"COMING_UP"}]`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.List(context.Background(), &resource.ListRequest{})
	if got := res.NativeIDs; len(got) != 2 || got[0] != "keep1" || got[1] != "keep2" {
		t.Fatalf("ids = %v", got)
	}
}

func TestProject_Read_GoneViaRemovedMessage(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = io.WriteString(w, `{"message":"Resource has been removed"}`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Read(context.Background(), &resource.ReadRequest{NativeID: "abc"})
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode = %v, want NotFound", res.ErrorCode)
	}
}

func TestProject_Read_GoneViaPrivileges(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = io.WriteString(w, `{"message":"Your account does not have the necessary privileges"}`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Read(context.Background(), &resource.ReadRequest{NativeID: "abc"})
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode = %v, want NotFound (project-scoped 403 fallback)", res.ErrorCode)
	}
}

func TestProject_Read_RealPermissionErrorIsAccessDenied(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = io.WriteString(w, `{"message":"plan does not support this feature"}`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Read(context.Background(), &resource.ReadRequest{NativeID: "abc"})
	if res.ErrorCode == resource.OperationErrorCodeNotFound {
		t.Fatalf("unrelated 403 must not be treated as gone (would nuke inventory)")
	}
}

func TestProject_Read_StatusRemovedTreatedAsGone(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"abc","status":"REMOVED"}`)
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Read(context.Background(), &resource.ReadRequest{NativeID: "abc"})
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode = %v", res.ErrorCode)
	}
}

func TestProject_Update_DiscardsNumericPatchID(t *testing.T) {
	// Project Update used to decode the PATCH response into projectAPI
	// (string `id`). Supabase actually returns a numeric internal id —
	// the decode would fail and the harness saw 'command reached terminal
	// state: Failed' after two minutes. Plugin now passes nil to Do() for
	// the PATCH and re-GETs to fill the response.
	var seen []string
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method)
		switch r.Method {
		case "PATCH":
			_, _ = io.WriteString(w, `{"id":1234,"name":"new"}`) // numeric id
		case "GET":
			_, _ = io.WriteString(w, `{"id":"abc","name":"new","status":"ACTIVE_HEALTHY"}`)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Update(context.Background(), &resource.UpdateRequest{
		NativeID:          "abc",
		DesiredProperties: []byte(`{"name":"new"}`),
	})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v err=%v", res.ProgressResult.OperationStatus, res.ProgressResult.StatusMessage)
	}
	if len(seen) != 2 || seen[0] != "PATCH" || seen[1] != "GET" {
		t.Errorf("call sequence = %v, want [PATCH GET]", seen)
	}
}

func TestProject_Update_RecordsManagedKeysIncrementally(t *testing.T) {
	managedKeysCache.Delete("proj-incr")
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/projects/proj-incr":
			_, _ = io.WriteString(w, `{"id":"proj-incr","status":"ACTIVE_HEALTHY"}`)
		case r.URL.Path == "/v1/projects/proj-incr/postgrest":
			w.WriteHeader(200)
		default:
			w.WriteHeader(200)
		}
	})
	defer srv.Close()
	p := &Project{Client: c}
	res, _ := p.Update(context.Background(), &resource.UpdateRequest{
		NativeID: "proj-incr",
		DesiredProperties: []byte(`{"api":{"settings":{"db_schema":"public","max_rows":1000}}}`),
	})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	got, ok := managedKeysCache.Load("proj-incr")
	if !ok {
		t.Fatalf("cache entry not written")
	}
	mk := got.(*projectManagedKeys)
	if _, hasKey := mk.api["db_schema"]; !hasKey {
		t.Errorf("api keys not recorded; mk.api = %v", mk.api)
	}
}

func TestMirrorDesiredBlocks(t *testing.T) {
	src := &ProjectProperties{
		API: &ConfigBlock{Settings: map[string]any{"k": "v"}},
	}
	dst := &ProjectProperties{Name: "n"}
	mirrorDesiredBlocks(src, dst)
	if dst.API == nil || dst.API.Settings["k"] != "v" {
		t.Errorf("API not mirrored")
	}
	if dst.Auth != nil || dst.Database != nil || dst.NetworkRestriction != nil {
		t.Errorf("nil blocks must not be set on dst")
	}
	if dst.Name != "n" {
		t.Errorf("scalar fields must not be touched")
	}
}

func TestFilterToKeys(t *testing.T) {
	in := map[string]any{"a": 1, "b": 2, "c": 3, "d": 4}
	keep := map[string]struct{}{"a": {}, "c": {}, "missing": {}}
	got := filterToKeys(in, keep)
	if len(got) != 2 || got["a"] != 1 || got["c"] != 3 {
		t.Errorf("got = %v", got)
	}
	if got := filterToKeys(in, nil); got != nil {
		t.Errorf("nil keep must return nil; got %v", got)
	}
	if got := filterToKeys(map[string]any{}, keep); got != nil {
		t.Errorf("all-missing must return nil; got %v", got)
	}
}

func TestIsProjectGone(t *testing.T) {
	apiErr := func(code int, msg string) error {
		return &supatransport.APIError{StatusCode: code, Message: msg}
	}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"400 removed verbatim", apiErr(400, "Resource has been removed"), true},
		{"400 'is being removed'", apiErr(400, "Project is being removed"), true},
		{"400 unrelated", apiErr(400, "bad request"), false},
		{"403 privileges", apiErr(403, "Your account does not have the necessary privileges"), true},
		{"403 unrelated", apiErr(403, "plan does not support this feature"), false},
		{"404", apiErr(404, "not found"), false}, // handled by IsNotFound, not this
		{"500", apiErr(500, "internal"), false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isProjectGone(tt.err); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
