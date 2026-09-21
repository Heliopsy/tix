// Package openproject imports work packages from an OpenProject instance.
package openproject

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
const System = core.SystemOpenProject

// Options configures one OpenProject import.
type Options struct {
	Config extsync.SourceConfig
	// UpdatedField is the dotted path holding each work package's update time.
	UpdatedField string
	Client       extsync.Doer
	Sleep        func(time.Duration)
	MaxRetries   int
	BaseDelay    time.Duration
}

// Importer reads work packages from OpenProject's API v3.
type Importer struct {
	http         *extsync.HTTPClient
	path         string
	filters      string
	updatedField string
	pageSize     int
	base         string
}

// collection is the part of an OpenProject collection this adapter reads.
type collection struct {
	Total    int `json:"total"`
	Count    int `json:"count"`
	Offset   int `json:"offset"`
	PageSize int `json:"pageSize"`
	Embedded struct {
		Elements []map[string]any `json:"elements"`
	} `json:"_embedded"`
}

// New builds an OpenProject importer.
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
	path := "/api/v3/work_packages"
	if project := strings.TrimSpace(opt.Config.Project); project != "" {
		path = "/api/v3/projects/" + url.PathEscape(project) + "/work_packages"
	}
	updated := opt.UpdatedField
	if updated == "" {
		updated = "updatedAt"
	}
	size := opt.Config.PageSize
	if size <= 0 {
		size = extsync.DefaultPageSize
	}
	return &Importer{
		http:         client,
		path:         path,
		filters:      strings.TrimSpace(opt.Config.Query),
		updatedField: updated,
		pageSize:     size,
		base:         strings.TrimRight(opt.Config.BaseURL, "/"),
	}, nil
}

// System names the external system these records came from.
func (i *Importer) System() string { return System }

// Attempts reports how many HTTP requests the import has issued, retries
// included.
func (i *Importer) Attempts() int { return i.http.Attempts() }

// Fetch returns one page of work packages ordered by update time, oldest first.
func (i *Importer) Fetch(ctx context.Context, opt extsync.Options) (extsync.Batch, error) {
	page := 1
	if opt.Page != "" {
		n, err := strconv.Atoi(opt.Page)
		if err != nil || n < 1 {
			return extsync.Batch{}, core.Invalid("page token %q is not an openproject page number", opt.Page)
		}
		page = n
	}
	size := extsync.PageSize(max(i.pageSize, opt.Size))

	query := url.Values{}
	query.Set("offset", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(size))
	query.Set("sortBy", `[["updatedAt","asc"]]`)
	if filters := i.filterSet(opt); filters != "" {
		query.Set("filters", filters)
	}

	var resp collection
	if err := i.http.GetJSON(ctx, i.path, query, &resp); err != nil {
		return extsync.Batch{}, err
	}

	batch := extsync.Batch{Records: make([]extsync.Record, 0, len(resp.Embedded.Elements))}
	for _, element := range resp.Embedded.Elements {
		r := extsync.Record{Fields: element}
		if raw, ok := extsync.Lookup(element, i.updatedField); ok {
			if parsed, ok := extsync.ParseTime(raw); ok {
				r.UpdatedAt = parsed
			}
		}
		if _, ok := element["url"]; !ok {
			if id, ok := extsync.Lookup(element, "id"); ok && i.base != "" {
				element["url"] = i.base + "/work_packages/" + extsync.Text(id)
			}
		}
		batch.Records = append(batch.Records, r)
	}

	if len(resp.Embedded.Elements) > 0 && page*size < resp.Total {
		batch.Page = strconv.Itoa(page + 1)
	}
	if n := len(batch.Records); n > 0 {
		if at := batch.Records[n-1].UpdatedAt; !at.IsZero() {
			batch.Cursor = at.UTC().Format(time.RFC3339Nano)
		}
	}
	return batch, nil
}

// filterSet narrows a refresh to work packages changed since the last
// successful run, on top of any configured filter.
func (i *Importer) filterSet(opt extsync.Options) string {
	if opt.Full || opt.Cursor == "" {
		return i.filters
	}
	since, ok := extsync.ParseTime(opt.Cursor)
	if !ok {
		return i.filters
	}
	clause := `{"updatedAt":{"operator":"<>d","values":["` + since.UTC().Format(time.RFC3339) + `",""]}}`
	if i.filters == "" {
		return "[" + clause + "]"
	}
	trimmed := strings.TrimSpace(i.filters)
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && len(trimmed) > 2 {
		return trimmed[:len(trimmed)-1] + "," + clause + "]"
	}
	return "[" + clause + "]"
}
