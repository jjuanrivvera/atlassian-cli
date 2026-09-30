// Command demoserver supplies invented API responses for an account-free VHS recording.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	addr := "127.0.0.1:8651"
	if len(args) > 0 {
		addr = args[0]
	}
	server := &http.Server{
		Addr: addr, Handler: demoHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	fmt.Fprintln(stderr, "demo API listening on", addr)
	return server.ListenAndServe()
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func demoHandler() http.Handler {
	issues := []map[string]any{
		{"id": "900101", "key": "DEMO-1", "fields": map[string]any{
			"summary": "Sketch the demo launch", "status": map[string]string{"name": "In Progress"},
			"priority": map[string]string{"name": "High"}, "issuetype": map[string]string{"name": "Task"},
			"project": map[string]string{"key": "DEMO"},
		}},
		{"id": "900102", "key": "DEMO-2", "fields": map[string]any{
			"summary": "Review the sample runbook", "status": map[string]string{"name": "To Do"},
			"priority": map[string]string{"name": "Medium"}, "issuetype": map[string]string{"name": "Task"},
			"project": map[string]string{"key": "DEMO"},
		}},
	}
	// The create and subsequent search share state, including when requests overlap.
	var mu sync.Mutex
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method + " " + r.URL.Path {
		case "GET /rest/api/3/search/jql":
			writeJSON(w, map[string]any{"issues": issues, "isLast": true})
		case "GET /rest/api/3/issue/DEMO-1":
			writeJSON(w, issues[0])
		case "POST /rest/api/3/issue":
			var body struct {
				Fields map[string]any `json:"fields"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			summary, _ := body.Fields["summary"].(string)
			if summary == "" {
				http.Error(w, "fields.summary is required", http.StatusBadRequest)
				return
			}
			fields := make(map[string]any, len(body.Fields)+2)
			for k, v := range body.Fields {
				fields[k] = v
			}
			fields["status"] = map[string]string{"name": "To Do"}
			if fields["priority"] == nil {
				fields["priority"] = map[string]string{"name": "Medium"}
			}
			issue := map[string]any{
				"id":           fmt.Sprint(900101 + len(issues)),
				"key":          fmt.Sprintf("DEMO-%d", len(issues)+1),
				"fields":       fields,
				"demo_request": body,
			}
			issues = append(issues, issue)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, issue)
		default:
			http.NotFound(w, r)
		}
	})
}
