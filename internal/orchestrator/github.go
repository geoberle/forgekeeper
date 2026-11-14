package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-github/v75/github"
)

// The SDK has no dependency endpoint in this version. Use its authenticated request directly.
func (o *Options) blockers(ctx context.Context, repo string, number int) ([]*github.Issue, error) {
	var all []*github.Issue
	for page := 1; page <= 1000; page++ {
		req, err := o.GitHub.NewRequest("GET", fmt.Sprintf("repos/%s/issues/%d/dependencies/blocked_by?per_page=100&page=%d", repo, number, page), nil)
		if err != nil {
			return nil, errors.New("invalid dependency request")
		}
		var items []*github.Issue
		_, err = o.GitHub.Do(ctx, req, &items)
		if err != nil {
			return nil, errors.New("GitHub dependency lookup failed")
		}
		all = append(all, items...)
		if len(items) < 100 {
			return all, nil
		}
	}
	return nil, errors.New("dependency pagination limit exceeded")
}
func failureMarker(id string) string { return "<!-- fabro-failure: " + id + " -->" }
func FailureComment(webURL, id, stage, retryUser string) string {
	return fmt.Sprintf("Fabro konnte diesen Versuch nicht erfolgreich abschließen.\n\nSchritt: `%s`\nRun: %s/runs/%s\n\nFür genau einen neuen Versuch: %s reagiert hier mit 🚀. Ready-Label, Abhängigkeiten und PR-Prüfung gelten weiterhin.\n\n%s", stage, webURL, id, retryUser, failureMarker(id))
}
func safeStage(state RunState) string {
	// Never copy an arbitrary node name or exception into a public comment.
	latest := ""
	for key, stage := range state.Stages {
		if stage.State != "failed" {
			continue
		}
		if len(latest) == 0 || stage.Completion.Timestamp.After(state.Stages[latest].Completion.Timestamp) {
			latest = key
		}
	}
	name, _, _ := strings.Cut(latest, "@")
	switch name {
	case "context", "implement", "result", "verify", "retry", "repair", "pr", "publish":
		return name
	}
	return "workflow"
}

func (o *Options) retry(ctx context.Context, c *Candidate) error {
	r := c.latest()
	if r == nil || !r.failed() || len(r.Labels["feedback_cursor"]) > 0 {
		return nil
	}
	owner, repo, _ := strings.Cut(c.Project.Repository, "/")
	for page := 1; page > 0; {
		comments, resp, err := o.GitHub.Issues.ListComments(ctx, owner, repo, c.Issue.GetNumber(), &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100, Page: page}})
		if err != nil {
			return errors.New("GitHub comment lookup failed")
		}
		for _, comment := range comments {
			if comment.GetUser().GetID() != o.BotID || !strings.HasSuffix(comment.GetBody(), failureMarker(r.ID)) {
				continue
			}
			c.CommentExists = true
			for rp := 1; rp > 0; {
				reactions, response, err := o.GitHub.Reactions.ListIssueCommentReactions(ctx, owner, repo, comment.GetID(), &github.ListReactionOptions{ListOptions: github.ListOptions{PerPage: 100, Page: rp}})
				if err != nil {
					return errors.New("GitHub reaction lookup failed")
				}
				for _, reaction := range reactions {
					if reaction.GetContent() == "rocket" && reaction.GetUser().GetID() == o.RetryUserID {
						c.Retry = true
					}
				}
				rp = response.NextPage
			}
		}
		page = resp.NextPage
	}
	return nil
}
func (o *Options) snapshot(ctx context.Context) ([]Candidate, []Run, error) {
	runs, err := o.Fabro.Runs(ctx)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Timestamps.CreatedAt.Equal(runs[j].Timestamps.CreatedAt) {
			return runs[i].ID > runs[j].ID
		}
		return runs[i].Timestamps.CreatedAt.After(runs[j].Timestamps.CreatedAt)
	})
	var candidates []Candidate
	for _, project := range o.Projects {
		owner, repo, _ := strings.Cut(project.Repository, "/")
		var prs []*github.PullRequest
		for page := 1; page > 0; {
			items, resp, err := o.GitHub.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{State: "all", ListOptions: github.ListOptions{PerPage: 100, Page: page}})
			if err != nil {
				return nil, nil, errors.New("GitHub PR lookup failed")
			}
			prs = append(prs, items...)
			page = resp.NextPage
		}
		var issues []*github.Issue
		for page := 1; page > 0; {
			items, resp, err := o.GitHub.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{State: "open", ListOptions: github.ListOptions{PerPage: 100, Page: page}})
			if err != nil {
				return nil, nil, errors.New("GitHub issue lookup failed")
			}
			issues = append(issues, items...)
			page = resp.NextPage
		}
		for _, issue := range issues {
			if issue.IsPullRequest() {
				continue
			}
			if len(o.raw.Repository) > 0 && !strings.EqualFold(project.Repository, o.raw.Repository) {
				continue
			}
			if o.raw.Issue > 0 && issue.GetNumber() != o.raw.Issue {
				continue
			}
			c := Candidate{Project: project, Issue: issue, PRs: prs}
			for _, r := range runs {
				if strings.EqualFold(r.Labels["github_issue"], c.key()) {
					if !runPattern.MatchString(r.ID) {
						return nil, nil, errors.New("invalid Fabro run identity")
					}
					c.Runs = append(c.Runs, r)
				}
			}
			c.Blockers, err = o.blockers(ctx, project.Repository, issue.GetNumber())
			if err != nil {
				return nil, nil, err
			}
			if r := c.latest(); r != nil {
				state, err := o.Fabro.State(ctx, r.ID)
				if err != nil {
					return nil, nil, err
				}
				if state.Status.Kind != r.Lifecycle.Status.Kind {
					return nil, nil, errors.New("fabro state changed; reconcile again")
				}
				c.Job, _ = state.Spec.Settings.Run.Inputs["job"].(string)
				if len(c.Job) > 0 && !jobPattern.MatchString(c.Job) {
					return nil, nil, errors.New("invalid Fabro job identity")
				}
				c.FailedStage = safeStage(state)
			}
			if err := o.retry(ctx, &c); err != nil {
				return nil, nil, err
			}
			candidates = append(candidates, c)
		}
	}
	return candidates, runs, nil
}
func (o *Options) postFailure(ctx context.Context, c Candidate) error {
	owner, repo, _ := strings.Cut(c.Project.Repository, "/")
	body := FailureComment(o.raw.WebURL, c.latest().ID, c.FailedStage, o.raw.RetryUser)
	_, _, err := o.GitHub.Issues.CreateComment(ctx, owner, repo, c.Issue.GetNumber(), &github.IssueComment{Body: github.Ptr(body)})
	if err != nil {
		return errors.New("GitHub failure comment not confirmed; reconcile on next poll")
	}
	return nil
}
