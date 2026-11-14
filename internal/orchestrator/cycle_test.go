package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v75/github"
)

func TestCyclePreviewCreateResumeAndFailure(t *testing.T) {
	for _, mode := range []string{"preview", "create", "ambiguous-create", "resume", "failure", "retry", "foreign-reaction", "old-reaction", "forged-comment", "github-error", "withdrawn"} {
		t.Run(mode, func(t *testing.T) {
			const id = "01M7ATV7XKXRZAXHWF2F104HQE"
			const job = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			storage := t.TempDir()
			if err := os.MkdirAll(filepath.Join(storage, job), 0700); err != nil {
				t.Fatal(err)
			}
			posts, starts, commands := 0, 0, 0
			issueReads := 0
			var commentBody string
			status := ""
			switch mode {
			case "resume":
				status = "submitted"
			case "failure", "retry", "foreign-reaction", "old-reaction", "forged-comment":
				status = "failed"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v1/runs":
					if len(status) == 0 {
						_, _ = fmt.Fprint(w, `{"data":[],"meta":{"has_more":false}}`)
						return
					}
					_, _ = fmt.Fprintf(w, `{"data":[{"id":%q,"labels":{"github_issue":"owner/repo#8"},"lifecycle":{"status":{"kind":%q}},"timestamps":{"created_at":"2026-11-13T10:00:00Z"}}],"meta":{"has_more":false}}`, id, status)
				case "/api/v1/runs/" + id + "/state":
					_, _ = fmt.Fprintf(w, `{"status":{"kind":%q},"spec":{"settings":{"run":{"inputs":{"job":%q}}}},"stages":{"verify@1":{"state":"failed"}}}`, status, job)
				case "/api/v1/runs/" + id + "/start":
					starts++
					w.WriteHeader(204)
				case "/repos/owner/repo/pulls":
					_, _ = fmt.Fprint(w, `[]`)
				case "/repos/owner/repo/issues":
					issueReads++
					if mode == "withdrawn" && issueReads > 1 {
						_, _ = fmt.Fprint(w, `[{"number":8,"state":"open","labels":[],"created_at":"2026-11-13T10:00:00Z"}]`)
						return
					}
					_, _ = fmt.Fprint(w, `[{"number":8,"state":"open","labels":[{"name":"ready-for-agent"}],"created_at":"2026-11-13T10:00:00Z"}]`)
				case "/repos/owner/repo/issues/8/dependencies/blocked_by":
					if mode == "github-error" {
						w.WriteHeader(500)
						_, _ = fmt.Fprint(w, `{"message":"synthetic-secret"}`)
					} else {
						_, _ = fmt.Fprint(w, `[]`)
					}
				case "/repos/owner/repo/issues/8/comments":
					if r.Method == "POST" {
						posts++
						var body struct{ Body string }
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						commentBody = body.Body
						_, _ = fmt.Fprint(w, `{"id":1}`)
						return
					}
					if mode == "failure" {
						_, _ = fmt.Fprint(w, `[]`)
						return
					}
					author := int64(10)
					marker := failureMarker(id)
					if mode == "forged-comment" {
						author = 99
					}
					if mode == "old-reaction" {
						marker = failureMarker("01M7AT8K7CXKDHSDBG7BH09AG1")
					}
					_, _ = fmt.Fprintf(w, `[{"id":12,"user":{"id":%d},"body":%q}]`, author, marker)
				case "/repos/owner/repo/issues/comments/12/reactions":
					author := int64(20)
					if mode == "foreign-reaction" {
						author = 99
					}
					_, _ = fmt.Fprintf(w, `[{"content":"rocket","user":{"id":%d}}]`, author)
				case "/repos/owner/repo":
					_, _ = fmt.Fprint(w, `{"default_branch":"main"}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			gh := github.NewClient(server.Client())
			gh.BaseURL, _ = url.Parse(server.URL + "/")
			opts := &Options{&completedOptions{raw: RawOptions{Preview: mode == "preview", Storage: storage, FabroURL: server.URL, WebURL: "http://localhost:32277", Workflow: "workflow.toml"}, Projects: []Project{{Repository: "owner/repo", Verification: []string{"make verify"}}}, GitHub: gh, Fabro: &FabroClient{HTTP: server.Client(), BaseURL: server.URL}, BotID: 10, RetryUserID: 20, Log: logr.Discard(), Execute: func(_ context.Context, name string, args []string, dir string) ([]byte, error) {
				commands++
				if name == "git" {
					if args[0] == "clone" {
						return nil, nil
					}
					return []byte("commit-sha"), nil
				}
				if mode == "ambiguous-create" {
					status = "submitted"
					return nil, errors.New("command failed")
				}
				joined := strings.Join(args, " ")
				if !strings.Contains(joined, "github_issue=owner/repo#8") {
					t.Error("missing issue identity")
				}
				return []byte(`{"run_id":"` + id + `"}`), nil
			}}}
			decisions, err := opts.Cycle(context.Background())
			if mode == "github-error" || mode == "ambiguous-create" {
				if err == nil || strings.Contains(err.Error(), "synthetic-secret") {
					t.Fatal("missing safe error")
				}
				if starts != 0 || posts != 0 {
					t.Fatal("mutation after uncertain read/create")
				}
				if mode == "ambiguous-create" {
					before := commands
					if _, err := opts.Cycle(context.Background()); err != nil {
						t.Fatal(err)
					}
					if commands != before || starts != 1 {
						t.Fatal("accepted create was not recovered")
					}
				}

				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "withdrawn":
				if commands != 0 || starts != 0 || posts != 0 {
					t.Fatal("stale readiness used")
				}
			case "preview":
				if commands != 0 || posts != 0 || starts != 0 || decisions[0].Action != "start" {
					t.Fatal("preview mutated state")
				}
			case "create", "retry":
				if commands != 3 || starts != 1 {
					t.Fatalf("commands %d starts %d", commands, starts)
				}
			case "resume":
				if commands != 0 || starts != 1 {
					t.Fatal("submitted run recreated")
				}
			case "failure", "old-reaction", "forged-comment":
				if posts != 1 || starts != 0 || commands != 0 || !strings.Contains(commentBody, failureMarker(id)) {
					t.Fatal("failure/retry identity broken")
				}
			case "foreign-reaction":
				if posts != 0 || starts != 0 || commands != 0 {
					t.Fatal("unauthorized retry")
				}
			}
		})
	}
}
