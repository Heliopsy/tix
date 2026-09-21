// Package jira imports issues from a Jira instance.
package jira

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

// System is the name recorded on every external reference this adapter writes.
const System = core.SystemJira

// SearchPath is the endpoint issues are read from.
const SearchPath = "/rest/api/3/search"

// cursorLayout is the timestamp format Jira Query Language accepts.
const cursorLayout = "2006-01-02 15:04"

// Options configures one Jira import.
type Options struct {
	Config extsync.SourceConfig
	// UpdatedField is the dotted path holding each issue's update time.
	UpdatedField string
	Client       extsync.Doer
	Sleep        func(time.Duration)
	MaxRetries   int
	BaseDelay    time.Duration
}

// Importer reads issues from Jira's search endpoint.
type Importer struct {
	http         *extsync.HTTPClient
	jql          string
	updatedField string
	pageSize     int
	browseBase   string
}

// searchResponse is the part of a Jira search answer this adapter reads.
type searchResponse struct {
	StartAt    int              `json:"startAt"`
	MaxResults int              `json:"maxResults"`
	Total      int              `json:"total"`
	IsLast     bool             `json:"isLast"`
	Issues     []map[string]any `json:"issues"`
}

// New builds a Jira importer.
func New(opt Options) (*Importer, error) {
	client, err := extsync.NewHTTPClient(extsync.HTTPOptions{
		System:     System,
		BaseURL:    opt.Config.BaseURL,
		Config:     opt.Config,
		Client:     opt.Client,
		Sleep:      opt.Sleep,
		MaxRetries: opt.MaxRetries,
		BaseDelay:  opt.BaseDelay,
	})
	if err != nil {
		return nil, err
	}
	jql := strings.TrimSpace(opt.Config.Query)
	if jql == "" && opt.Config.Project != "" {
		jql = "project = " + strconv.Quote(opt.Config.Project)
	}
	if jql == "" {
		return nil, core.Invalid("a jira source needs a project or a jql query; set %s or %s",
			extsync.EnvPrefix+"<SOURCE>_"+extsync.EnvProject, extsync.EnvPrefix+"<SOURCE>_"+extsync.EnvQuery)
	}
	updated := opt.UpdatedField
	if updated == "" {
		updated = "fields.updated"
	}
	size := opt.Config.PageSize
	if size <= 0 {
		size = extsync.DefaultPageSize
	}
	return &Importer{
		http:         client,
		jql:          jql,
		updatedField: updated,
		pageSize:     size,
		browseBase:   strings.TrimRight(opt.Config.BaseURL, "/"),
	}, nil
}

// System names the external system these records came from.
func (i *Importer) System() string { return System }

// Attempts reports how many HTTP requests the import has issued, retries
// included.
func (i *Importer) Attempts() int { return i.http.Attempts() }

// Fetch returns one page of issues ordered by update time, oldest first.
func (i *Importer) Fetch(ctx context.Context, opt extsync.Options) (extsync.Batch, error) {
	startAt := 0
	if opt.Page != "" {
		n, err := strconv.Atoi(opt.Page)
		if err != nil || n < 0 {
			return extsync.Batch{}, core.Invalid("page token %q is not a jira offset", opt.Page)
		}
		startAt = n
	}
	size := extsync.PageSize(max(i.pageSize, opt.Size))

	query := url.Values{}
	query.Set("jql", i.query(opt))
	query.Set("startAt", strconv.Itoa(startAt))
	query.Set("maxResults", strconv.Itoa(size))

	var resp searchResponse
	if err := i.http.GetJSON(ctx, SearchPath, query, &resp); err != nil {
		return extsync.Batch{}, err
	}

	batch := extsync.Batch{Records: make([]extsync.Record, 0, len(resp.Issues))}
	for _, issue := range resp.Issues {
		r := extsync.Record{Fields: issue}
		if raw, ok := extsync.Lookup(issue, i.updatedField); ok {
			if parsed, ok := extsync.ParseTime(raw); ok {
				r.UpdatedAt = parsed
			}
		}
		if _, ok := issue["url"]; !ok {
			if key, ok := issue["key"].(string); ok && i.browseBase != "" {
				issue["url"] = i.browseBase + "/browse/" + key
			}
		}
		batch.Records = append(batch.Records, r)
	}

	next := startAt + len(resp.Issues)
	if len(resp.Issues) > 0 && !resp.IsLast && next < resp.Total {
		batch.Page = strconv.Itoa(next)
	}
	if n := len(batch.Records); n > 0 {
		if at := batch.Records[n-1].UpdatedAt; !at.IsZero() {
			batch.Cursor = at.UTC().Format(time.RFC3339Nano)
		}
	}
	return batch, nil
}

// query narrows the configured JQL to what changed since the last successful
// run, and orders results so a committed cursor cannot skip an issue.
func (i *Importer) query(opt extsync.Options) string {
	jql := i.jql
	if !opt.Full && opt.Cursor != "" {
		if since, ok := extsync.ParseTime(opt.Cursor); ok {
			jql = "(" + jql + ") AND updated >= " + strconv.Quote(since.UTC().Format(cursorLayout))
		}
	}
	return jql + " ORDER BY updated ASC"
}
