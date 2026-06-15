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
	"testing"

	"github.com/platform-engineering-labs/formae/pkg/plugin/resource"
)

// The Supabase secrets endpoint is a single bulk bag per project. We model it
// as one resource (SUPABASE::Functions::Secrets) holding all names→values, so
// every mutation is a single atomic bulk call — no concurrent read-modify-write
// race between sibling secret resources.
//
// The bag's native id is "{projectRef}/secrets", NOT the bare project ref: the
// Project resource's native id is the bare ref ($.id), and formae keys
// resources by (target, nativeID) regardless of type. A bare-ref bag would
// collide with its own project and get deleted as a duplicate during sync.

func TestSecrets_Create_PostsAllAtOnce(t *testing.T) {
	var got []map[string]string
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/projects/p1/secrets" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(201)
	})
	defer srv.Close()
	s := &Secrets{Client: c}
	props, _ := json.Marshal(SecretsProperties{
		ProjectRef: "p1",
		Values:     map[string]string{"OPENAI": "sk-1", "WEBHOOK": "wh-2"},
	})
	res, _ := s.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	// Distinct from the Project's native id ("p1") to avoid the collision.
	if res.ProgressResult.NativeID != "p1/secrets" {
		t.Fatalf("NativeID = %q, want p1/secrets", res.ProgressResult.NativeID)
	}
	// One POST carrying both secrets, sorted by name for determinism.
	if len(got) != 2 || got[0]["name"] != "OPENAI" || got[1]["name"] != "WEBHOOK" {
		t.Fatalf("body = %v, want [OPENAI, WEBHOOK]", got)
	}
}

func TestSecrets_Create_NativeIDNotBareProjectRef(t *testing.T) {
	// Regression: the bag must never reuse the bare project ref as its native
	// id, or it collides with the SUPABASE::Platform::Project resource.
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201) })
	defer srv.Close()
	s := &Secrets{Client: c}
	props, _ := json.Marshal(SecretsProperties{ProjectRef: "p1", Values: map[string]string{"A": "1"}})
	res, _ := s.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.NativeID == "p1" {
		t.Fatalf("NativeID == bare project ref %q — collides with Project", res.ProgressResult.NativeID)
	}
}

func TestSecrets_Create_RejectsSupabasePrefix(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("API must not be called") })
	defer srv.Close()
	s := &Secrets{Client: c}
	props, _ := json.Marshal(SecretsProperties{
		ProjectRef: "p1",
		Values:     map[string]string{"SUPABASE_X": "v"},
	})
	res, _ := s.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.ErrorCode != resource.OperationErrorCodeInvalidRequest {
		t.Fatalf("ErrorCode = %v, want InvalidRequest", res.ProgressResult.ErrorCode)
	}
}

func TestSecrets_Create_RejectsEmpty(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("API must not be called") })
	defer srv.Close()
	s := &Secrets{Client: c}
	props, _ := json.Marshal(SecretsProperties{ProjectRef: "p1", Values: map[string]string{}})
	res, _ := s.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.ErrorCode != resource.OperationErrorCodeInvalidRequest {
		t.Fatalf("ErrorCode = %v, want InvalidRequest", res.ProgressResult.ErrorCode)
	}
}

func TestSecrets_Read_ReturnsProjectRef(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/p1/secrets" {
			t.Errorf("path = %q, want /v1/projects/p1/secrets", r.URL.Path)
		}
		_, _ = io.WriteString(w, `[{"name":"OPENAI"},{"name":"SUPABASE_URL"}]`)
	})
	defer srv.Close()
	s := &Secrets{Client: c}
	res, _ := s.Read(context.Background(), &resource.ReadRequest{NativeID: "p1/secrets"})
	if res.ErrorCode != "" {
		t.Fatalf("ErrorCode = %v", res.ErrorCode)
	}
	var p SecretsProperties
	_ = json.Unmarshal([]byte(res.Properties), &p)
	if p.ProjectRef != "p1" {
		t.Fatalf("projectRef = %q, want p1", p.ProjectRef)
	}
}

func TestSecrets_Read_NotFoundWhenEmpty(t *testing.T) {
	// After an out-of-band delete the project's secrets endpoint still returns
	// 200 with only reserved SUPABASE_* entries. With no managed secrets left,
	// the bag must read as NotFound so formae clears it from inventory.
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"name":"SUPABASE_URL"},{"name":"SUPABASE_ANON_KEY"}]`)
	})
	defer srv.Close()
	s := &Secrets{Client: c}
	res, _ := s.Read(context.Background(), &resource.ReadRequest{NativeID: "p1/secrets"})
	if res.ErrorCode != resource.OperationErrorCodeNotFound {
		t.Fatalf("ErrorCode = %v, want NotFound", res.ErrorCode)
	}
}

func TestSecrets_Update_UpsertsAndDeletesRemoved(t *testing.T) {
	var posted []map[string]string
	var deleted []string
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/p1/secrets" {
			t.Errorf("path = %q", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		switch r.Method {
		case "POST":
			_ = json.Unmarshal(b, &posted)
		case "DELETE":
			_ = json.Unmarshal(b, &deleted)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
		w.WriteHeader(200)
	})
	defer srv.Close()
	s := &Secrets{Client: c}
	prior, _ := json.Marshal(SecretsProperties{ProjectRef: "p1", Values: map[string]string{"A": "1", "B": "2"}})
	desired, _ := json.Marshal(SecretsProperties{ProjectRef: "p1", Values: map[string]string{"A": "1-new", "C": "3"}})
	res, _ := s.Update(context.Background(), &resource.UpdateRequest{
		NativeID:          "p1/secrets",
		PriorProperties:   prior,
		DesiredProperties: desired,
	})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	// Desired set upserted (sorted): A, C.
	if len(posted) != 2 || posted[0]["name"] != "A" || posted[1]["name"] != "C" {
		t.Fatalf("posted = %v, want [A, C]", posted)
	}
	// B was removed from the bag → deleted.
	if len(deleted) != 1 || deleted[0] != "B" {
		t.Fatalf("deleted = %v, want [B]", deleted)
	}
}

func TestSecrets_Delete_RemovesAllNonReserved(t *testing.T) {
	var deleted []string
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/p1/secrets" {
			t.Errorf("path = %q", r.URL.Path)
		}
		switch r.Method {
		case "GET":
			_, _ = io.WriteString(w, `[{"name":"SUPABASE_URL"},{"name":"FOO"},{"name":"BAR"}]`)
		case "DELETE":
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &deleted)
			w.WriteHeader(200)
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})
	defer srv.Close()
	s := &Secrets{Client: c}
	res, _ := s.Delete(context.Background(), &resource.DeleteRequest{NativeID: "p1/secrets"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
	// Reserved SUPABASE_* left alone; managed names deleted, sorted.
	if len(deleted) != 2 || deleted[0] != "BAR" || deleted[1] != "FOO" {
		t.Fatalf("deleted = %v, want [BAR, FOO]", deleted)
	}
}

func TestSecrets_List_OnePerProject(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/projects" {
			_, _ = io.WriteString(w, `[{"id":"p1"},{"id":"p2"}]`)
			return
		}
		_, _ = io.WriteString(w, `[{"name":"FOO"}]`)
	})
	defer srv.Close()
	s := &Secrets{Client: c}
	res, _ := s.List(context.Background(), &resource.ListRequest{})
	// One Secrets bag per project, native id "{ref}/secrets" — distinct from
	// the Project's bare-ref native id.
	if len(res.NativeIDs) != 2 || res.NativeIDs[0] != "p1/secrets" || res.NativeIDs[1] != "p2/secrets" {
		t.Fatalf("NativeIDs = %v, want [p1/secrets, p2/secrets]", res.NativeIDs)
	}
}
