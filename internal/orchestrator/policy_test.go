package orchestrator

import (
	"os"
	"testing"
	"time"

	"github.com/google/go-github/v75/github"
)

func TestDecide(t *testing.T) {
	tests := []struct {
		name, state, label, blocker, run, pr string
		retry                                bool
		action, reason                       string
	}{
		{name: "ready", state: "open", label: "ready-for-agent", action: "start", reason: "eligible"},
		{name: "experiment label", state: "open", label: "ready-for-test", action: "start", reason: "eligible"},
		{name: "closed", state: "closed", label: "ready-for-agent", action: "wait", reason: "issue_closed"},
		{name: "no readiness", state: "open", action: "wait", reason: "not_ready"},
		{name: "planning", state: "open", label: "wayfinder:map", action: "wait", reason: "planning"},
		{name: "dependency", state: "open", label: "ready-for-agent", blocker: "open", action: "wait", reason: "blocked"},
		{name: "unknown dependency", state: "open", label: "ready-for-agent", blocker: "unknown", action: "wait", reason: "blocked"},
		{name: "merged dependency", state: "open", label: "ready-for-agent", blocker: "closed", action: "start", reason: "eligible"},
		{name: "running", state: "open", label: "ready-for-agent", run: "running", action: "wait", reason: "active_run"},
		{name: "resume submitted", state: "open", label: "ready-for-agent", run: "submitted", action: "resume", reason: "submitted"},
		{name: "failed", state: "open", label: "ready-for-agent", run: "failed", action: "wait", reason: "retry_required"},
		{name: "retry", state: "open", label: "ready-for-agent", run: "failed", retry: true, action: "start", reason: "eligible"},
		{name: "retry still blocked", state: "open", label: "ready-for-agent", run: "failed", retry: true, blocker: "open", action: "wait", reason: "blocked"},
		{name: "open draft", state: "open", label: "ready-for-agent", pr: "open", action: "wait", reason: "open_pr"},
		{name: "success without PR", state: "open", label: "ready-for-agent", run: "succeeded", action: "wait", reason: "pr_not_reconciled"},
		{name: "rejected PR without metadata", state: "open", label: "ready-for-agent", run: "failed", pr: "closed", retry: true, action: "wait", reason: "closed_unmerged_pr"},
		{name: "merged partial", state: "open", label: "ready-for-agent", run: "succeeded", pr: "merged", action: "start", reason: "eligible"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Candidate{Project: Project{Repository: "owner/repo"}, Issue: &github.Issue{Number: github.Ptr(8), State: github.Ptr(tt.state), Labels: []*github.Label{{Name: github.Ptr(tt.label)}}}, Job: "job", Retry: tt.retry}
			if len(tt.blocker) > 0 {
				c.Blockers = []*github.Issue{{State: github.Ptr(tt.blocker)}}
			}
			if len(tt.run) > 0 {
				r := Run{ID: "run"}
				r.Lifecycle.Status.Kind = tt.run
				c.Runs = []Run{r}
			}
			if len(tt.pr) > 0 {
				pr := &github.PullRequest{Number: github.Ptr(43), State: github.Ptr(tt.pr), Draft: github.Ptr(true), Body: github.Ptr("Part of #8"), Head: &github.PullRequestBranch{Ref: github.Ptr("fabro/issue-8-job"), Repo: &github.Repository{FullName: github.Ptr("owner/repo")}}}
				if tt.pr == "merged" {
					pr.State = github.Ptr("closed")
					pr.MergedAt = &github.Timestamp{Time: time.Now()}
				}
				c.PRs = []*github.PullRequest{pr}
			}
			got := Decide(c)
			if got.Action != tt.action || got.Reason != tt.reason {
				t.Fatalf("got %+v want %s/%s", got, tt.action, tt.reason)
			}
		})
	}
}
func TestPlanOldestAndGlobalCapacity(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(map[bool]string{false: "oldest first", true: "other run occupies server"}[active], func(t *testing.T) {
			a := Candidate{Project: Project{Repository: "owner/new"}, Issue: &github.Issue{Number: github.Ptr(1), State: github.Ptr("open"), Labels: []*github.Label{{Name: github.Ptr("ready-for-agent")}}, CreatedAt: &github.Timestamp{Time: time.Unix(200, 0)}}}
			b := Candidate{Project: Project{Repository: "owner/old"}, Issue: &github.Issue{Number: github.Ptr(2), State: github.Ptr("open"), Labels: []*github.Label{{Name: github.Ptr("ready-for-agent")}}, CreatedAt: &github.Timestamp{Time: time.Unix(100, 0)}}}
			var runs []Run
			if active {
				r := Run{}
				r.Lifecycle.Status.Kind = "running"
				runs = []Run{r}
			}
			got := Plan([]Candidate{a, b}, runs)
			if got[0].Repository != "owner/old" {
				t.Fatal(got)
			}
			want := "start"
			if active {
				want = "wait"
			}
			if got[0].Action != want || got[1].Action != "wait" {
				t.Fatal(got)
			}
		})
	}
}
func TestIssueReferences(t *testing.T) {
	for _, tt := range []struct {
		body string
		want bool
	}{{"Part of #8", true}, {"Fixes owner/repo#8", true}, {"https://github.com/owner/repo/issues/8", true}, {"#80", false}, {"other/repo#8", false}, {"https://github.com/owner/repo/issues/80", false}} {
		if got := references(&github.PullRequest{Body: github.Ptr(tt.body)}, "owner/repo", 8); got != tt.want {
			t.Errorf("%q: %v", tt.body, got)
		}
	}
}
func TestFailureCommentGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/failure.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := FailureComment("http://localhost:32277", "01M7ATV7XKXRZAXHWF2F104HQE", "verify", "retry-reviewer") + "\n"; got != string(data) {
		t.Fatalf("unexpected comment:\n%s", got)
	}
}
