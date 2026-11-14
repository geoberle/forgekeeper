package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunsPaginationAndErrors(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					_, _ = fmt.Fprint(w, `{"data":[{"id":"first","lifecycle":{"status":{"kind":"failed"}}}],"meta":{"has_more":true}}`)
					return
				}
				if r.URL.Query().Get("page[offset]") != "100" || r.URL.Query().Get("include_archived") != "true" {
					t.Error("pagination or archived flag missing")
				}
				if bad {
					w.WriteHeader(500)
					_, _ = fmt.Fprint(w, "synthetic-secret")
					return
				}
				_, _ = fmt.Fprint(w, `{"data":[{"id":"last","lifecycle":{"status":{"kind":"running"}}}],"meta":{"has_more":false}}`)
			}))
			defer server.Close()
			client := FabroClient{HTTP: server.Client(), BaseURL: server.URL, Token: "synthetic-secret"}
			got, err := client.Runs(context.Background())
			if bad {
				if err == nil || got != nil || strings.Contains(err.Error(), "synthetic-secret") {
					t.Fatal("partial snapshot or credential leak")
				}
			} else if err != nil || len(got) != 2 {
				t.Fatalf("%v %v", got, err)
			}
		})
	}
}
func TestAmbiguousStartReconcilesKnownRun(t *testing.T) {
	for _, state := range []string{"running", "submitted"} {
		t.Run(state, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					w.WriteHeader(409)
					_, _ = fmt.Fprint(w, "synthetic-secret")
					return
				}
				_, _ = fmt.Fprintf(w, `{"status":{"kind":%q}}`, state)
			}))
			defer server.Close()
			client := FabroClient{HTTP: server.Client(), BaseURL: server.URL}
			err := client.Start(context.Background(), "run")
			if (err == nil) != (state == "running") {
				t.Fatalf("state %s: %v", state, err)
			}
		})
	}
}

func TestIncompleteListsAreNotEmptySnapshots(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":[],"meta":{}}`, `{"data":null,"meta":{"has_more":false}}`, `{"data":[],"meta":{"has_more":true}}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, body) }))
			defer server.Close()
			client := FabroClient{HTTP: server.Client(), BaseURL: server.URL}
			if _, err := client.Runs(context.Background()); err == nil {
				t.Fatal("incomplete list accepted")
			}
		})
	}
}
