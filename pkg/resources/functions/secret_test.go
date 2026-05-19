// © 2026 Platform Engineering Labs Inc.
//
// SPDX-License-Identifier: Apache-2.0

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

func TestSecret_Create_PostsArray(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/projects/p1/secrets" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var got []map[string]string
		_ = json.Unmarshal(b, &got)
		if len(got) != 1 || got[0]["name"] != "OPENAI" {
			t.Errorf("body = %v", got)
		}
		w.WriteHeader(201)
	})
	defer srv.Close()
	s := &Secret{Client: c}
	props, _ := json.Marshal(SecretProperties{ProjectRef: "p1", Name: "OPENAI", Value: "sk-xyz"})
	res, _ := s.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.NativeID != "p1/OPENAI" {
		t.Fatalf("NativeID = %q", res.ProgressResult.NativeID)
	}
}

func TestSecret_RejectsSupabasePrefix(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) { t.Error("API must not be called") })
	defer srv.Close()
	s := &Secret{Client: c}
	props, _ := json.Marshal(SecretProperties{ProjectRef: "p1", Name: "SUPABASE_X", Value: "v"})
	res, _ := s.Create(context.Background(), &resource.CreateRequest{Properties: props})
	if res.ProgressResult.ErrorCode != resource.OperationErrorCodeInvalidRequest {
		t.Fatalf("ErrorCode = %v", res.ProgressResult.ErrorCode)
	}
}

func TestSecret_Read_FindsName(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"name":"OTHER"},{"name":"OPENAI"}]`)
	})
	defer srv.Close()
	s := &Secret{Client: c}
	res, _ := s.Read(context.Background(), &resource.ReadRequest{NativeID: "p1/OPENAI"})
	if res.ErrorCode != "" {
		t.Fatalf("ErrorCode = %v", res.ErrorCode)
	}
}

func TestSecret_Delete_BulkArray(t *testing.T) {
	c, srv := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		var names []string
		_ = json.Unmarshal(b, &names)
		if len(names) != 1 || names[0] != "OPENAI" {
			t.Errorf("body = %v", names)
		}
	})
	defer srv.Close()
	s := &Secret{Client: c}
	res, _ := s.Delete(context.Background(), &resource.DeleteRequest{NativeID: "p1/OPENAI"})
	if res.ProgressResult.OperationStatus != resource.OperationStatusSuccess {
		t.Fatalf("status = %v", res.ProgressResult.OperationStatus)
	}
}
