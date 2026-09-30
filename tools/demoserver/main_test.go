package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestDemoHandlerRecordedReads(t *testing.T) {
	h := demoHandler()
	for _, path := range []string{"/rest/api/3/search/jql?jql=project%3DDEMO", "/rest/api/3/issue/DEMO-1"} {
		rec := request(h, http.MethodGet, path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body)
		}
		if rec.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("GET %s did not return JSON", path)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if path == "/rest/api/3/issue/DEMO-1" {
			if body["key"] != "DEMO-1" || body["fields"].(map[string]any)["summary"] != "Sketch the demo launch" {
				t.Fatalf("unexpected issue: %s", rec.Body)
			}
		} else {
			if len(body["issues"].([]any)) != 2 || body["isLast"] != true {
				t.Fatalf("unexpected search: %s", rec.Body)
			}
		}
	}
}

func TestDemoHandlerEchoesCreateAndListsIt(t *testing.T) {
	h := demoHandler()
	payload := `{"fields":{"project":{"key":"DEMO"},"issuetype":{"name":"Task"},"summary":"Publish the demo checklist","labels":["demo"]}}`
	rec := request(h, http.MethodPost, "/rest/api/3/issue", payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST issue = %d: %s", rec.Code, rec.Body)
	}
	var created, sent map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(payload), &sent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(created["demo_request"], sent) {
		t.Fatalf("create did not echo request: %s", rec.Body)
	}
	if created["key"] != "DEMO-3" {
		t.Fatalf("unexpected key: %v", created["key"])
	}
	fields := created["fields"].(map[string]any)
	for k, want := range sent["fields"].(map[string]any) {
		if !reflect.DeepEqual(fields[k], want) {
			t.Errorf("field %s = %v, want %v", k, fields[k], want)
		}
	}
	rec = request(h, http.MethodGet, "/rest/api/3/search/jql", "")
	var search struct {
		Issues []map[string]any `json:"issues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &search); err != nil {
		t.Fatal(err)
	}
	if len(search.Issues) != 3 || !reflect.DeepEqual(search.Issues[2], created) {
		t.Fatalf("created issue missing from next listing: %s", rec.Body)
	}
	// A fresh recording must start with the original fixtures again.
	rec = request(demoHandler(), http.MethodGet, "/rest/api/3/search/jql", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &search); err != nil {
		t.Fatal(err)
	}
	if len(search.Issues) != 2 {
		t.Fatalf("state leaked between recordings: %s", rec.Body)
	}
}

func TestDemoHandlerInvalidCreateDoesNotChangeListing(t *testing.T) {
	h := demoHandler()
	for _, payload := range []string{"not JSON", "null", `{}`, `{"fields":{"summary":42}}`} {
		rec := request(h, http.MethodPost, "/rest/api/3/issue", payload)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s = %d, want 400", payload, rec.Code)
		}
	}
	rec := request(h, http.MethodGet, "/rest/api/3/search/jql", "")
	var body struct {
		Issues []any `json:"issues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Issues) != 2 {
		t.Fatalf("invalid create changed state: %s", rec.Body)
	}
}

func TestDemoHandlerUnknownRoutes(t *testing.T) {
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/unknown"}, {http.MethodGet, "/rest/api/3/issue"},
		{http.MethodPost, "/rest/api/3/search/jql"}, {http.MethodDelete, "/rest/api/3/issue/DEMO-1"},
	} {
		rec := request(demoHandler(), route.method, route.path, "")
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", route.method, route.path, rec.Code)
		}
	}
}

func TestDemoServerRejectsInvalidAddress(t *testing.T) {
	var log bytes.Buffer
	// Invalid syntax fails before opening a socket, even in the recording sandbox.
	err := run([]string{"invalid::address"}, &log)
	if err == nil || !strings.Contains(err.Error(), "too many colons") {
		t.Fatalf("invalid listen address returned %v", err)
	}
	if !strings.Contains(log.String(), "demo API listening on invalid::address") {
		t.Fatalf("missing startup diagnostic: %s", &log)
	}
}
