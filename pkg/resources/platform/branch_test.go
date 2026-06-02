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
	"testing"

	supatransport "github.com/platform-engineering-labs/formae-plugin-supabase/pkg/transport/supabase"
	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

func TestBranch_Create(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/parent1/branches" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"CREATING_PROJECT"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	props, _ := json.Marshal(BranchProperties{ParentProjectRef: "parent1", BranchName: "feat"})
	res, _ := b.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.NativeID != "parent1/branch-uuid" {
		t.Fatalf("NativeID = %q", res.ProgressResult.NativeID)
	}
}

func TestBranch_Status_FunctionsDeployed(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"FUNCTIONS_DEPLOYED"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Status(context.Background(), &resource.StatusRequest{RequestID: "parent1/branch-uuid"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestBranch_Status_Failure(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"MIGRATIONS_FAILED"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Status(context.Background(), &resource.StatusRequest{RequestID: "parent1/branch-uuid"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusFailure {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestBranch_Read_UsesGlobalEndpoint(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/branches/branch-uuid" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"FUNCTIONS_DEPLOYED"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Read(context.Background(), &resource.ReadRequest{NativeID: "parent1/branch-uuid"})
	if res.ErrorCode != "" {
		t.Fatalf("ErrorCode %v", res.ErrorCode)
	}
}

func TestBranch_Status_ActiveHealthy(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"ACTIVE_HEALTHY"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Status(context.Background(), &resource.StatusRequest{RequestID: "parent1/branch-uuid"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}

func TestBranch_Delete_Unpersists(t *testing.T) {
	var calls []string
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch len(calls) {
		case 1: // first DELETE: 422
			w.WriteHeader(422)
			_, _ = io.WriteString(w, `{"message":"Cannot delete persistent branch."}`)
		case 2: // PATCH unpersist: ok
			w.WriteHeader(200)
			_, _ = io.WriteString(w, `{}`)
		case 3: // retry DELETE: ok
			w.WriteHeader(204)
		default:
			t.Fatalf("unexpected extra call: %s", r.Method+" "+r.URL.Path)
		}
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Delete(context.Background(), &resource.DeleteRequest{NativeID: "parent1/branch-uuid"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	want := []string{
		"DELETE /v1/branches/branch-uuid",
		"PATCH /v1/branches/branch-uuid",
		"DELETE /v1/branches/branch-uuid",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v", calls)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], w)
		}
	}
}

func TestBranch_Delete_NoUnpersistOnUnrelated422(t *testing.T) {
	var calls int
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(422)
		_, _ = io.WriteString(w, `{"message":"some other validation error"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Delete(context.Background(), &resource.DeleteRequest{NativeID: "parent1/branch-uuid"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusFailure {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry for unrelated 422)", calls)
	}
}

func TestBranch_Update_OmitsPersistentWhenAbsent(t *testing.T) {
	var gotBody map[string]any
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = io.WriteString(w, `{"id":"branch-uuid","branch_name":"renamed"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	// DesiredProperties only sets branch_name — no persistent key.
	res, _ := b.Update(context.Background(), &resource.UpdateRequest{
		NativeID:          "parent1/branch-uuid",
		DesiredProperties: []byte(`{"branch_name":"renamed"}`),
	})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	if _, ok := gotBody["persistent"]; ok {
		t.Errorf("body must omit persistent when user did not set it; got %v", gotBody)
	}
	if gotBody["branch_name"] != "renamed" {
		t.Errorf("body branch_name = %v", gotBody["branch_name"])
	}
}

func TestBranch_Update_NoMutableFieldsTriggersGET(t *testing.T) {
	var method string
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		_, _ = io.WriteString(w, `{"id":"branch-uuid","status":"ACTIVE_HEALTHY"}`)
	})
	defer srv.Close()
	b := &Branch{Client: c}
	res, _ := b.Update(context.Background(), &resource.UpdateRequest{
		NativeID:          "parent1/branch-uuid",
		DesiredProperties: []byte(`{}`),
	})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	if method != "GET" {
		t.Errorf("method = %q, want GET (no body fields)", method)
	}
}

func TestIsCannotDeletePersistent(t *testing.T) {
	apiErr := func(code int, msg string) error {
		return &supatransport.APIError{StatusCode: code, Message: msg}
	}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", io.EOF, false},
		{"422 verbatim", apiErr(422, "Cannot delete persistent branch."), true},
		{"422 case-insensitive", apiErr(422, "cannot delete persistent something"), true},
		{"422 other", apiErr(422, "validation failed"), false},
		{"409 persistent", apiErr(409, "Cannot delete persistent branch."), false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCannotDeletePersistent(tt.err); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

