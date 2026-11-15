package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v75/github"
)

func TestFeedbackCursor(t *testing.T) {
	for _, tt := range []struct {
		input string
		valid bool
	}{{"", true}, {"1/2/3", true}, {"-1/2/3", false}, {"1/2", false}, {"x/2/3", false}} {
		t.Run(tt.input, func(t *testing.T) {
			c, err := parseFeedbackCursor(tt.input)
			if (err == nil) != tt.valid {
				t.Fatalf("cursor: %v", err)
			}
			if tt.valid && len(tt.input) > 0 && c.String() != tt.input {
				t.Fatal(c)
			}
		})
	}
}
func TestFeedbackPending(t *testing.T) {
	for _, tt := range []struct {
		name    string
		kind    string
		author  int64
		id      int64
		pending bool
		state   string
	}{
		{"new comment", "comment", 20, 11, true, ""},
		{"already attempted", "comment", 20, 10, false, ""},
		{"own bot", "comment", 10, 11, false, ""},
		{"external author", "comment", 30, 11, false, ""},
		{"inline review", "inline", 20, 11, true, ""},
		{"submitted review", "review", 20, 11, true, "CHANGES_REQUESTED"},
		{"empty approval", "review", 20, 11, true, "APPROVED"},
		{"pending review", "review", 20, 11, false, "PENDING"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := feedback{}
			user := &github.User{ID: github.Ptr(tt.author)}
			switch tt.kind {
			case "comment":
				f.Comments = []*github.IssueComment{{ID: github.Ptr(tt.id), User: user}}
			case "inline":
				f.Inline = []*github.PullRequestComment{{ID: github.Ptr(tt.id), User: user}}
			case "review":
				f.Reviews = []*github.PullRequestReview{{ID: github.Ptr(tt.id), User: user, State: github.Ptr(tt.state)}}
			}
			next, pending := f.pending(feedbackCursor{10, 10, 10}, 20, 10)
			if pending != tt.pending {
				t.Fatalf("pending=%v", pending)
			}
			if _, again := f.pending(next, 20, 10); again {
				t.Fatal("event processed twice")
			}
		})
	}
}
func TestFeedbackArrivingDuringRunRemainsPending(t *testing.T) {
	before := feedback{Comments: []*github.IssueComment{{ID: github.Ptr(int64(10)), User: &github.User{ID: github.Ptr(int64(20))}}}}
	cursor, pending := before.pending(feedbackCursor{}, 20, 10)
	if !pending {
		t.Fatal("initial event missing")
	}
	after := feedback{Comments: []*github.IssueComment{{ID: github.Ptr(int64(10)), User: &github.User{ID: github.Ptr(int64(20))}}, {ID: github.Ptr(int64(11)), User: &github.User{ID: github.Ptr(int64(20))}}}}
	if next, pending := after.pending(cursor, 20, 10); !pending || next.Comments != 11 {
		t.Fatal("new event lost")
	}
}
func TestFeedbackCycleRespectsActiveRun(t *testing.T) {
	o := &Options{&completedOptions{}}
	r := Run{}
	r.Lifecycle.Status.Kind = "running"
	d, err := o.feedbackCycle(context.Background(), []Run{r})
	if err != nil || d != nil {
		t.Fatalf("%v %v", d, err)
	}
}
func TestFeedbackCyclePreviewManagedPR(t *testing.T) {
	for _, state := range []string{"open", "closed", "foreign", "foreign-author", "api-error"} {
		t.Run(state, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/owner/repo/pulls/48":
					if state == "api-error" {
						w.WriteHeader(503)
						return
					}
					repo := "owner/repo"
					if state == "foreign" {
						repo = "other/repo"
					}
					prState := "open"
					if state == "closed" {
						prState = "closed"
					}
					author := int64(10)
					if state == "foreign-author" {
						author = 30
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"id": author}, "number": 48, "state": prState, "head": map[string]any{"repo": map[string]string{"full_name": repo}}})
				case "/repos/owner/repo/issues/48/comments":
					_, _ = w.Write([]byte(`[{"id":12,"user":{"id":20},"body":"Please fix this"}]`))
				case "/repos/owner/repo/pulls/48/reviews", "/repos/owner/repo/pulls/48/comments":
					_, _ = w.Write([]byte(`[]`))
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			gh := github.NewClient(server.Client())
			gh.BaseURL, _ = gh.BaseURL.Parse(server.URL + "/")
			o := &Options{&completedOptions{raw: RawOptions{Preview: true}, Projects: []Project{{Repository: "owner/repo"}}, GitHub: gh, RetryUserID: 20, BotID: 10}}
			r := Run{Labels: map[string]string{"github_issue": "owner/repo#8", "github_pr": "48", "feedback_cursor": "11/0/0"}}
			r.Lifecycle.Status.Kind = "failed"
			d, err := o.feedbackCycle(context.Background(), []Run{r})
			if state == "api-error" {
				if err == nil {
					t.Fatal("API failure swallowed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if state == "open" {
				if d == nil || d.Action != "feedback" {
					t.Fatalf("%v", d)
				}
			} else if d != nil {
				t.Fatal("closed/foreign PR selected")
			}
		})
	}
}

func TestLaunchFeedbackRecordsCursorAndUsesExistingBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v1/runs/01M7DDESZ5V7SF59PYYEJ7HCEG/start" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	storage := t.TempDir()
	var createArgs []string
	o := &Options{&completedOptions{raw: RawOptions{Storage: storage, Workflow: "/experiment/workflows/implement-issue/workflow.toml", FabroURL: server.URL}, Fabro: &FabroClient{HTTP: server.Client(), BaseURL: server.URL}, RetryUserID: 20, Execute: func(_ context.Context, name string, args []string, _ string) ([]byte, error) {
		if name == "fabro" {
			createArgs = args
			return []byte(`{"run_id":"01M7DDESZ5V7SF59PYYEJ7HCEG"}`), nil
		}
		if args[0] == "rev-parse" && args[1] == "HEAD" {
			return []byte("head"), nil
		}
		if args[0] == "rev-parse" {
			return []byte("tree"), nil
		}
		if args[0] == "merge-base" {
			return []byte("base"), nil
		}
		if args[0] == "clone" && args[2] != "--branch" {
			t.Fatalf("unexpected clone args: %v", args)
		}
		return nil, nil
	}}}
	pr := &github.PullRequest{Number: github.Ptr(48), Head: &github.PullRequestBranch{Ref: github.Ptr("fabro/existing"), SHA: github.Ptr("head")}, Base: &github.PullRequestBranch{Ref: github.Ptr("main"), SHA: github.Ptr("base")}}
	if err := o.launchFeedback(context.Background(), Project{Repository: "owner/repo"}, 8, pr, feedback{}, feedbackCursor{1, 2, 3}, feedbackCursor{4, 5, 6}); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(createArgs, " ")
	for _, expected := range []string{"followup-pr/workflow.toml", "github_pr=48", "feedback_cursor=4/5/6", "github_issue=owner/repo#8"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %s in %v", expected, createArgs)
		}
	}
	jobs, err := os.ReadDir(storage)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs: %v %v", jobs, err)
	}
	data, err := os.ReadFile(filepath.Join(storage, jobs[0].Name(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile("testdata/feedback-config.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatalf("feedback configuration differs from golden:\n%s", data)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config["branch"] != "fabro/existing" || config["initial_tree"] != "tree" || config["expected_head"] != "head" || config["base_sha"] != "base" {
		t.Fatal(config)
	}
}

func TestFeedbackCycleScopesActiveRunsToPR(t *testing.T) {
	for _, tt := range []struct {
		name, repository, issue, pr string
		pending, checkMerge         bool
	}{
		{"same PR", "owner/repo", "8", "48", false, false},
		{"same issue before PR association", "owner/repo", "8", "", false, false},
		{"other issue", "owner/repo", "9", "49", false, true},
		{"other PR same issue", "owner/repo", "8", "49", false, true},
		{"same number other repository", "other/repo", "8", "48", false, true},
		{"pending feedback waits for capacity", "owner/repo", "9", "49", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mergeChecked := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/owner/repo/pulls/48":
					_, _ = w.Write([]byte(`{"number":48,"state":"open","mergeable":true,"mergeable_state":"clean","user":{"id":10},"base":{"repo":{"full_name":"owner/repo"}},"head":{"sha":"head","repo":{"full_name":"owner/repo"}}}`))
				case "/repos/owner/repo/issues/48/comments":
					if tt.pending {
						_, _ = w.Write([]byte(`[{"id":12,"user":{"id":20},"body":"Fix this"}]`))
					} else {
						_, _ = w.Write([]byte(`[]`))
					}
				case "/repos/owner/repo/pulls/48/reviews", "/repos/owner/repo/pulls/48/comments":
					_, _ = w.Write([]byte(`[]`))
				case "/api/v1/runs/completed/state":
					mergeChecked = true
					// No publication evidence: exercise eligibility without allowing a merge.
					_, _ = w.Write([]byte(`{"spec":{"settings":{"run":{"inputs":{}}}}}`))
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			gh := github.NewClient(server.Client())
			gh.BaseURL, _ = gh.BaseURL.Parse(server.URL + "/")
			o := &Options{&completedOptions{raw: RawOptions{Preview: true}, Projects: []Project{{Repository: "owner/repo", AutoMerge: true}}, GitHub: gh, RetryUserID: 20, BotID: 10}}
			o.Fabro = &FabroClient{BaseURL: server.URL, HTTP: server.Client()}
			completed := Run{ID: "completed", Labels: map[string]string{"github_issue": "owner/repo#8", "github_pr": "48"}}
			completed.Lifecycle.Status.Kind = "succeeded"
			active := Run{ID: "active", Labels: map[string]string{"github_issue": tt.repository + "#" + tt.issue, "github_pr": tt.pr}}
			active.Lifecycle.Status.Kind = "running"
			d, err := o.feedbackCycle(context.Background(), []Run{active, completed})
			if err != nil || d != nil {
				t.Fatalf("decision=%v err=%v", d, err)
			}
			if mergeChecked != tt.checkMerge {
				t.Fatalf("merge checked=%v, want %v", mergeChecked, tt.checkMerge)
			}
		})
	}
}
