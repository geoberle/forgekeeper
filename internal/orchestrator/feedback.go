package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/v75/github"
)

type feedbackCursor struct{ Comments, Reviews, Inline int64 }

func (c feedbackCursor) String() string {
	return fmt.Sprintf("%d/%d/%d", c.Comments, c.Reviews, c.Inline)
}
func parseFeedbackCursor(s string) (feedbackCursor, error) {
	var c feedbackCursor
	if len(s) == 0 {
		return c, nil
	}
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return c, errors.New("invalid feedback cursor")
	}
	for i, p := range parts {
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil || n < 0 {
			return c, errors.New("invalid feedback cursor")
		}
		switch i {
		case 0:
			c.Comments = n
		case 1:
			c.Reviews = n
		case 2:
			c.Inline = n
		}
	}
	return c, nil
}

type feedback struct {
	Comments []*github.IssueComment       `json:"comments"`
	Reviews  []*github.PullRequestReview  `json:"reviews"`
	Inline   []*github.PullRequestComment `json:"inline_comments"`
}

func (f feedback) pending(cursor feedbackCursor, userID, botID int64) (feedbackCursor, bool) {
	next := cursor
	pending := false
	for _, c := range f.Comments {
		if c.GetID() > cursor.Comments && c.GetUser().GetID() == userID && userID != botID {
			pending = true
		}
		next.Comments = max(next.Comments, c.GetID())
	}
	for _, r := range f.Reviews {
		if r.GetState() == "PENDING" {
			continue
		}
		if r.GetID() > cursor.Reviews && r.GetUser().GetID() == userID && userID != botID {
			pending = true
		}
		next.Reviews = max(next.Reviews, r.GetID())
	}
	for _, c := range f.Inline {
		if c.GetID() > cursor.Inline && c.GetUser().GetID() == userID && userID != botID {
			pending = true
		}
		next.Inline = max(next.Inline, c.GetID())
	}
	return next, pending
}
func (o *Options) readFeedback(ctx context.Context, owner, repo string, number int) (feedback, error) {
	var f feedback
	for page := 1; page > 0; {
		items, res, err := o.GitHub.Issues.ListComments(ctx, owner, repo, number, &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return f, errors.New("PR comment lookup failed")
		}
		f.Comments = append(f.Comments, items...)
		page = res.NextPage
	}
	for page := 1; page > 0; {
		items, res, err := o.GitHub.PullRequests.ListReviews(ctx, owner, repo, number, &github.ListOptions{PerPage: 100, Page: page})
		if err != nil {
			return f, errors.New("PR review lookup failed")
		}
		f.Reviews = append(f.Reviews, items...)
		page = res.NextPage
	}
	for page := 1; page > 0; {
		items, res, err := o.GitHub.PullRequests.ListComments(ctx, owner, repo, number, &github.PullRequestListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return f, errors.New("PR inline comment lookup failed")
		}
		f.Inline = append(f.Inline, items...)
		page = res.NextPage
	}
	return f, nil
}

// Run labels are the durable consumption boundary: a failed attempt needs new human
// feedback, not an infinite retry. All history remains available to subsequent runs.
func (o *Options) feedbackCycle(ctx context.Context, runs []Run) (*Decision, error) {
	active := 0
	for _, r := range runs {
		if !r.terminal() {
			active++
		}
	}
	if active > 1 {
		return nil, nil
	}
	for _, r := range runs {
		if !r.terminal() {
			if r.Lifecycle.Status.Kind == "submitted" && len(r.Labels["feedback_cursor"]) > 0 {
				d := &Decision{Action: "resume", Reason: "submitted_feedback", RunID: r.ID}
				if o.raw.Preview {
					return d, nil
				}
				state, err := o.Fabro.State(ctx, r.ID)
				if err != nil {
					return nil, err
				}
				job, _ := state.Spec.Settings.Run.Inputs["job"].(string)
				if !jobPattern.MatchString(job) {
					return nil, errors.New("invalid feedback job identity")
				}
				if err := writeJSON(filepath.Join(o.raw.Storage, job, "run.json"), map[string]string{"run_id": r.ID}); err != nil {
					return nil, err
				}
				return d, o.Fabro.Start(ctx, r.ID)
			}
			return nil, nil
		}
	}
	for _, project := range o.Projects {
		if len(o.raw.Repository) > 0 && !strings.EqualFold(project.Repository, o.raw.Repository) {
			continue
		}
		owner, repo, _ := strings.Cut(project.Repository, "/")
		managed := map[int]string{}
		cursors := map[int]feedbackCursor{}
		latestFeedback := map[int]Run{}
		latestRun := map[int]Run{}
		for _, r := range runs {
			issueRepo, issueText, ok := strings.Cut(r.Labels["github_issue"], "#")
			if !ok || !strings.EqualFold(issueRepo, project.Repository) {
				continue
			}
			issue, err := strconv.Atoi(issueText)
			if err != nil || issue <= 0 {
				continue
			}
			if o.raw.Issue > 0 && issue != o.raw.Issue {
				continue
			}
			number := 0
			if r.PullRequest != nil {
				number = r.PullRequest.Number
			}
			if s := r.Labels["github_pr"]; len(s) > 0 {
				number, err = strconv.Atoi(s)
				if err != nil {
					return nil, errors.New("invalid PR identity")
				}
			}
			if number <= 0 {
				continue
			}
			if old, ok := managed[number]; ok && old != issueText {
				return nil, errors.New("conflicting PR issue identity")
			}
			managed[number] = issueText
			if old, ok := latestRun[number]; !ok || r.Timestamps.CreatedAt.After(old.Timestamps.CreatedAt) || (r.Timestamps.CreatedAt.Equal(old.Timestamps.CreatedAt) && r.ID > old.ID) {
				latestRun[number] = r
			}
			if len(r.Labels["feedback_cursor"]) > 0 {
				old, exists := latestFeedback[number]
				if !exists || r.Timestamps.CreatedAt.After(old.Timestamps.CreatedAt) ||
					(r.Timestamps.CreatedAt.Equal(old.Timestamps.CreatedAt) && r.ID > old.ID) {
					latestFeedback[number] = r
				}
			}
			c, err := parseFeedbackCursor(r.Labels["feedback_cursor"])
			if err != nil {
				return nil, err
			}
			old := cursors[number]
			cursors[number] = feedbackCursor{max(c.Comments, old.Comments), max(c.Reviews, old.Reviews), max(c.Inline, old.Inline)}
		}
		numbers := make([]int, 0, len(managed))
		for n := range managed {
			numbers = append(numbers, n)
		}
		sort.Ints(numbers)
		for _, number := range numbers {
			pr, _, err := o.GitHub.PullRequests.Get(ctx, owner, repo, number)
			if err != nil {
				return nil, errors.New("managed PR lookup failed")
			}
			if pr.GetState() != "open" || pr.GetMerged() || o.BotID == 0 || pr.GetUser().GetID() != o.BotID {
				continue
			}
			if !strings.EqualFold(pr.GetHead().GetRepo().GetFullName(), project.Repository) {
				continue
			}
			f, err := o.readFeedback(ctx, owner, repo, number)
			if err != nil {
				return nil, err
			}
			if failed, ok := latestFeedback[number]; ok && failed.failed() && !o.raw.Preview {
				found := false
				for _, comment := range f.Comments {
					if comment.GetUser().GetID() == o.BotID && strings.HasSuffix(comment.GetBody(), failureMarker(failed.ID)) {
						found = true
					}
				}
				if !found {
					body := fmt.Sprintf("Die PR-Folgearbeit ist fehlgeschlagen. Run: %s/runs/%s\n\nBitte prüfe den Run und schreibe einen neuen Kommentar für einen weiteren Versuch.\n\n%s", o.raw.WebURL, failed.ID, failureMarker(failed.ID))
					if _, _, err := o.GitHub.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{Body: github.Ptr(body)}); err != nil {
						return nil, errors.New("PR failure comment not confirmed")
					}
				}
			}
			cursor, pending := f.pending(cursors[number], o.RetryUserID, o.BotID)
			if !pending {
				if project.AutoMerge {
					if err := o.mergePR(ctx, project, pr, latestRun[number], f); err != nil {
						return nil, err
					}
				}
				continue
			}
			issue, _ := strconv.Atoi(managed[number])
			d := &Decision{Repository: project.Repository, Issue: issue, Action: "feedback", Reason: fmt.Sprintf("pr_%d", number)}
			if o.raw.Preview {
				return d, nil
			}
			return d, o.launchFeedback(ctx, project, issue, pr, f, cursors[number], cursor)
		}
	}
	return nil, nil
}
func (o *Options) launchFeedback(ctx context.Context, project Project, issue int, pr *github.PullRequest, f feedback, previous, cursor feedbackCursor) error {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return errors.New("cannot allocate feedback job")
	}
	job := hex.EncodeToString(id[:])
	dir := filepath.Join(o.raw.Storage, job)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.New("cannot prepare feedback job")
	}
	source := filepath.Join(dir, "source")
	if _, err := o.Execute(ctx, "git", []string{"clone", "--single-branch", "--branch", pr.GetHead().GetRef(), "https://github.com/" + project.Repository + ".git", source}, ""); err != nil {
		return errors.New("PR checkout failed")
	}
	head, err := o.Execute(ctx, "git", []string{"rev-parse", "HEAD"}, source)
	if err != nil || strings.TrimSpace(string(head)) != pr.GetHead().GetSHA() {
		return errors.New("PR head changed; reconcile again")
	}
	if _, err := o.Execute(ctx, "git", []string{"fetch", "origin", pr.GetBase().GetRef()}, source); err != nil {
		return errors.New("cannot fetch PR base")
	}
	base, err := o.Execute(ctx, "git", []string{"merge-base", "HEAD", "FETCH_HEAD"}, source)
	if err != nil {
		return errors.New("cannot resolve PR diff base")
	}
	tree, err := o.Execute(ctx, "git", []string{"rev-parse", "HEAD^{tree}"}, source)
	if err != nil {
		return errors.New("cannot resolve PR tree")
	}
	config := map[string]any{"repository": project.Repository, "instructions": project.Instructions, "verification": project.Verification, "issue": issue, "base": pr.GetBase().GetRef(), "base_sha": strings.TrimSpace(string(base)), "pr": pr.GetNumber(), "branch": pr.GetHead().GetRef(), "expected_head": pr.GetHead().GetSHA(), "initial_tree": strings.TrimSpace(string(tree)), "feedback_user_id": o.RetryUserID, "feedback_previous": previous, "feedback_cursor": cursor}
	if err := writeJSON(filepath.Join(dir, "config.json"), config); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(dir, "feedback.json"), f); err != nil {
		return err
	}
	workflow := filepath.Join(filepath.Dir(filepath.Dir(o.raw.Workflow)), "followup-pr", "workflow.toml")
	output, err := o.Execute(ctx, "fabro", []string{"create", workflow, "--json", "--server", o.raw.FabroURL, "--environment", "local", "--target-from", source, "-I", "repository=" + project.Repository, "-I", fmt.Sprintf("issue=%d", issue), "-I", "job=" + job, "--label", fmt.Sprintf("github_issue=%s#%d", project.Repository, issue), "--label", fmt.Sprintf("github_pr=%d", pr.GetNumber()), "--label", "feedback_cursor=" + cursor.String()}, "")
	if err != nil {
		return errors.New("feedback create not confirmed; reconcile on next poll")
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	if json.Unmarshal(output, &created) != nil || !runPattern.MatchString(created.RunID) {
		return errors.New("invalid feedback run identity")
	}
	if err := writeJSON(filepath.Join(dir, "run.json"), map[string]string{"run_id": created.RunID}); err != nil {
		return err
	}
	return o.Fabro.Start(ctx, created.RunID)
}
