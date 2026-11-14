package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-github/v75/github"
)

func TestLowRisk(t *testing.T) {
	for _, tt := range []struct {
		name, change string
		allowed      bool
	}{
		{"two low risk reviews", "", true}, {"missing reviewer", "missing", false}, {"high risk", "high", false}, {"unknown risk", "unknown", false}, {"missing reason", "reason", false}, {"human needed", "human", false}, {"blocking finding", "blocking", false}, {"missing findings", "findings", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var r mergeEvidence
			if err := json.Unmarshal([]byte(`{"reviewers":{"steve":{"status":"complete","risk":"low","risk_reason":"bounded and covered","findings":[]},"david":{"status":"complete","risk":"low","risk_reason":"bounded and covered","findings":[]}}}`), &r); err != nil {
				t.Fatal(err)
			}
			v := r.Reviewers["steve"]
			switch tt.change {
			case "high":
				v.Risk = "high"
			case "unknown":
				v.Risk = ""
			case "reason":
				v.Reason = ""
			case "human":
				v.Status = "needs_human"
			case "blocking":
				_ = json.Unmarshal([]byte(`[{"severity":"blocking"}]`), &v.Findings)
			case "findings":
				v.Findings = nil
			}
			r.Reviewers["steve"] = v
			if tt.change == "missing" {
				delete(r.Reviewers, "david")
			}
			if lowRisk(r) != tt.allowed {
				t.Fatal("unexpected risk decision")
			}
		})
	}
}

func TestMergePR(t *testing.T) {
	for _, tt := range []struct {
		name             string
		private, preview bool
		change           string
		merge, auto      bool
	}{
		{"private merge", true, false, "", true, false}, {"public auto merge", false, false, "", false, true},
		{"foreign author", true, false, "author", false, false}, {"draft", true, false, "draft", false, false},
		{"new head", true, false, "head", false, false}, {"changed base", true, false, "base", false, false},
		{"missing check", true, false, "missing-check", false, false}, {"failed check", true, false, "failed-check", false, false},
		{"wrong check app", true, false, "app", false, false}, {"changes requested", true, false, "review", false, false},
		{"high risk", true, false, "risk", false, false}, {"preview", true, true, "", false, false},
		{"public missing protection", false, false, "protection", false, false},
		{"private missing evidence", true, false, "evidence", false, false}, {"failed run", true, false, "failed-run", false, false},
		{"head moved during checks", true, false, "moved", false, false},
		{"second page failed", true, false, "page", false, false},
		{"skipped check", true, false, "skipped", false, false},
		{"late human review", true, false, "late-review", false, false},
		{"pending risk status on public", false, false, "blocked", false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			job := "0123456789abcdef0123456789abcdef"
			runID := "01M7DDESZ5V7SF59PYYEJ7HCEG"
			files := map[string]string{
				"config":         `{"repository":"owner/repo","base":"main","base_sha":"base"}`,
				"run":            `{"run_id":"` + runID + `"}`,
				"publication":    `{"mode":"ready","publish_head":"head","tree":"tree"}`,
				"review-request": `{"head":"reviewed","tree":"tree"}`,
				"verified":       `{"tree":"tree"}`, "review-approved": `{"tree":"tree"}`,
				"review": `{"head":"reviewed","reviewers":{"steve":{"status":"complete","risk":"low","risk_reason":"tested","findings":[]},"david":{"status":"complete","risk":"low","risk_reason":"tested","findings":[]}}}`,
			}
			for name, data := range files {
				var v any
				if err := json.Unmarshal([]byte(data), &v); err != nil {
					t.Fatal(err)
				}
				if name == "review" && tt.change == "risk" {
					v.(map[string]any)["reviewers"].(map[string]any)["steve"].(map[string]any)["risk"] = "high"
				}
				if name == "verified" && tt.change == "evidence" {
					continue
				}
				if err := os.MkdirAll(filepath.Join(dir, job), 0700); err != nil {
					t.Fatal(err)
				}
				if err := writeJSON(filepath.Join(dir, job, name+".json"), v); err != nil {
					t.Fatal(err)
				}
			}
			pr := &github.PullRequest{Number: github.Ptr(5), NodeID: github.Ptr("node"), State: github.Ptr("open"), User: &github.User{ID: github.Ptr(int64(10))}, Mergeable: github.Ptr(true), MergeableState: github.Ptr("clean"), Head: &github.PullRequestBranch{SHA: github.Ptr("head"), Repo: &github.Repository{FullName: github.Ptr("owner/repo")}}, Base: &github.PullRequestBranch{SHA: github.Ptr("base"), Ref: github.Ptr("main"), Repo: &github.Repository{FullName: github.Ptr("owner/repo")}}}
			switch tt.change {
			case "author":
				pr.User.ID = github.Ptr(int64(30))
			case "blocked":
				pr.MergeableState = github.Ptr("blocked")
			case "draft":
				pr.Draft = github.Ptr(true)
			case "head":
				pr.Head.SHA = github.Ptr("other")
			case "base":
				pr.Base.SHA = github.Ptr("other")
			}
			f := feedback{}
			if tt.change == "review" {
				f.Reviews = []*github.PullRequestReview{{ID: github.Ptr(int64(1)), User: &github.User{ID: github.Ptr(int64(20))}, State: github.Ptr("CHANGES_REQUESTED")}}
			}
			merged, auto := false, false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/runs/" + runID + "/state":
					_, _ = w.Write([]byte(`{"spec":{"settings":{"run":{"inputs":{"job":"` + job + `"}}}}}`))
				case "/repos/owner/repo/git/commits/head":
					_, _ = w.Write([]byte(`{"tree":{"sha":"tree"}}`))
				case "/repos/owner/repo/commits/head/check-runs":
					checks := []map[string]any{{"name": "CI", "status": "completed", "conclusion": "success", "app": map[string]string{"slug": "github-actions"}}}
					switch tt.change {
					case "page":
						if r.URL.Query().Get("page") == "1" {
							w.Header().Set("Link", "<http://"+r.Host+r.URL.Path+"?page=2>; rel=\"next\"")
						} else {
							checks[0]["conclusion"] = "failure"
						}
					case "skipped":
						checks[0]["conclusion"] = "skipped"
					case "missing-check":
						checks = nil
					case "failed-check":
						checks[0]["conclusion"] = "failure"
					case "app":
						checks[0]["app"] = map[string]string{"slug": "other"}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"check_runs": checks})
				case "/repos/owner/repo/commits/head/status":
					_, _ = w.Write([]byte(`{"statuses":[]}`))
				case "/repos/owner/repo":
					_ = json.NewEncoder(w).Encode(map[string]any{"private": tt.private, "allow_merge_commit": true, "allow_auto_merge": true})
				case "/repos/owner/repo/pulls/5":
					if tt.change == "moved" {
						pr.Head.SHA = github.Ptr("new")
					}
					_ = json.NewEncoder(w).Encode(pr)
				case "/repos/owner/repo/pulls/5/reviews":
					if tt.change == "late-review" {
						_, _ = w.Write([]byte(`[{"id":42,"user":{"id":20},"state":"CHANGES_REQUESTED"}]`))
					} else {
						_, _ = w.Write([]byte(`[]`))
					}
				case "/repos/owner/repo/issues/5/comments", "/repos/owner/repo/pulls/5/comments":
					_, _ = w.Write([]byte(`[]`))
				case "/repos/owner/repo/pulls/5/merge":
					var input map[string]any
					_ = json.NewDecoder(r.Body).Decode(&input)
					if input["sha"] != "head" || input["merge_method"] != "merge" {
						t.Error("merge not bound to approved head")
					}
					merged = true
					_, _ = w.Write([]byte(`{"merged":true}`))
				case "/repos/owner/repo/branches/main/protection":
					contexts := []string{riskContext, "CI"}
					if tt.change == "protection" {
						contexts = []string{"CI"}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"required_status_checks": map[string]any{"strict": true, "contexts": contexts}})
				case "/repos/owner/repo/statuses/head":
					_, _ = w.Write([]byte(`{}`))
				case "/graphql":
					var input struct {
						Variables map[string]string `json:"variables"`
					}
					_ = json.NewDecoder(r.Body).Decode(&input)
					if input.Variables["head"] != "head" {
						t.Error("auto merge not bound to approved head")
					}
					auto = true
					_, _ = w.Write([]byte(`{"data":{"enablePullRequestAutoMerge":{"pullRequest":{"id":"node"}}}}`))
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			gh := github.NewClient(server.Client())
			gh.BaseURL, _ = gh.BaseURL.Parse(server.URL + "/")
			o := &Options{&completedOptions{raw: RawOptions{Storage: dir, Preview: tt.preview}, GitHub: gh, Fabro: &FabroClient{HTTP: server.Client(), BaseURL: server.URL}, BotID: 10, RetryUserID: 20, Log: logr.Discard()}}
			run := Run{ID: runID}
			run.Lifecycle.Status.Kind = "succeeded"
			if tt.change == "failed-run" {
				run.Lifecycle.Status.Kind = "failed"
			}
			err := o.mergePR(context.Background(), Project{Repository: "owner/repo", AutoMerge: true, MergeChecks: []string{"CI"}}, pr, run, f)
			if err != nil {
				t.Fatal(err)
			}
			if merged != tt.merge || auto != tt.auto {
				t.Fatalf("merged=%v auto=%v", merged, auto)
			}
		})
	}
}
