// Package lever fetches a Lever account's public postings.
package lever

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/normalize"
)

const (
	Base   = "https://api.lever.co"
	EUBase = "https://api.eu.lever.co"
)

type Fetcher struct {
	board  boards.Board
	client *httpjson.Client
	base   string
}

// New serves a board with a company name, since Lever's API omits it. Region
// "eu" selects euBase; an account's host is per company, not per API.
func New(b boards.Board, c *httpjson.Client, base, euBase string) (*Fetcher, error) {
	if b.Company == "" {
		return nil, fmt.Errorf("lever %s: company is required", b.Slug)
	}
	switch b.Region {
	case "":
	case "eu":
		base = euBase
	default:
		return nil, fmt.Errorf("lever %s: unknown region %q", b.Slug, b.Region)
	}
	return &Fetcher{board: b, client: c, base: base}, nil
}

func (f *Fetcher) Name() string   { return f.board.Name() }
func (f *Fetcher) Rendered() bool { return false }

type job struct {
	ID            string `json:"id"`
	Text          string `json:"text"`
	HostedURL     string `json:"hostedUrl"`
	CreatedAt     int64  `json:"createdAt"`
	WorkplaceType string `json:"workplaceType"`
	Description   string `json:"description"`
	Additional    string `json:"additional"`
	Lists         []struct {
		Text    string `json:"text"`
		Content string `json:"content"`
	} `json:"lists"`
	Categories struct {
		Location string `json:"location"`
	} `json:"categories"`
}

// Fetch reads every posting in one call. Lever splits a body into an intro,
// titled lists and a closing section; Fetch joins them in that order.
func (f *Fetcher) Fetch(ctx context.Context) ([]normalize.Posting, error) {
	var jobs []job
	url := fmt.Sprintf("%s/v0/postings/%s?mode=json", f.base, f.board.Slug)
	if err := f.client.Get(ctx, url, &jobs); err != nil {
		return nil, fmt.Errorf("lever %s: %w", f.board.Slug, err)
	}
	out := make([]normalize.Posting, 0, len(jobs))
	for _, j := range jobs {
		p := normalize.New(f.board.Source, f.board.Slug, j.ID)
		p.Company = f.board.Company
		p.Title = j.Text
		p.Location = j.Categories.Location
		p.Remote = j.WorkplaceType == "remote" || normalize.RemoteIn(j.Categories.Location)
		p.URL = j.HostedURL
		p.DescriptionHTML = body(j)
		p.DescriptionText = normalize.Text(p.DescriptionHTML)
		if j.CreatedAt > 0 {
			p.PostedAt = normalize.Stamp(time.UnixMilli(j.CreatedAt))
		}
		out = append(out, p)
	}
	return out, nil
}

func body(j job) string {
	var b strings.Builder
	b.WriteString(j.Description)
	for _, l := range j.Lists {
		fmt.Fprintf(&b, "<h3>%s</h3><ul>%s</ul>", l.Text, l.Content)
	}
	b.WriteString(j.Additional)
	return b.String()
}
