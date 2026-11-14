package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type FabroClient struct {
	HTTP    *http.Client
	BaseURL string
	Token   string
}

// Never surface response bodies, URLs or underlying transport errors: they can carry credentials.
func (c *FabroClient) request(ctx context.Context, method, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/api/v1"+path, bytes.NewBufferString("{}"))
	if err != nil {
		return errors.New("invalid Fabro request")
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errors.New("fabro request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("fabro HTTP %d", resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(out); err != nil {
			return errors.New("invalid Fabro response")
		}
	}
	return nil
}
func (c *FabroClient) Runs(ctx context.Context) ([]Run, error) {
	var runs []Run
	for offset := 0; offset < 100000; offset += 100 {
		var page struct {
			Data []Run `json:"data"`
			Meta *struct {
				HasMore *bool `json:"has_more"`
			} `json:"meta"`
		}
		path := fmt.Sprintf("/runs?include_archived=true&page[limit]=100&page[offset]=%d", offset)
		if err := c.request(ctx, "GET", path, &page); err != nil {
			return nil, err
		}
		if page.Data == nil || page.Meta == nil || page.Meta.HasMore == nil {
			return nil, errors.New("incomplete Fabro list")
		}
		for _, r := range page.Data {
			if len(r.ID) == 0 || len(r.Lifecycle.Status.Kind) == 0 {
				return nil, errors.New("incomplete Fabro run")
			}
		}
		runs = append(runs, page.Data...)
		if !*page.Meta.HasMore {
			return runs, nil
		}
		if len(page.Data) == 0 {
			return nil, errors.New("incomplete Fabro pagination")
		}
	}
	return nil, errors.New("fabro pagination limit exceeded")
}

type RunState struct {
	Status struct {
		Kind string `json:"kind"`
	} `json:"status"`
	Spec struct {
		Settings struct {
			Run struct {
				Inputs map[string]any `json:"inputs"`
			} `json:"run"`
		} `json:"settings"`
	} `json:"spec"`
	Stages map[string]RunStage `json:"stages"`
}

type RunStage struct {
	State      string `json:"state"`
	Completion struct {
		Timestamp time.Time `json:"timestamp"`
	} `json:"completion"`
}

func (c *FabroClient) State(ctx context.Context, id string) (RunState, error) {
	var s RunState
	err := c.request(ctx, "GET", "/runs/"+url.PathEscape(id)+"/state", &s)
	return s, err
}
func (c *FabroClient) Start(ctx context.Context, id string) error {
	err := c.request(ctx, "POST", "/runs/"+url.PathEscape(id)+"/start", nil)
	if err == nil {
		return nil
	}
	s, readErr := c.State(ctx, id)
	if readErr == nil {
		switch s.Status.Kind {
		case "running", "queued", "blocked", "succeeded", "failed", "dead":
			return nil
		}
	}
	return errors.New("fabro start not confirmed; reconcile on next poll")
}
