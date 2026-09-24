// Package greenhouse fetches a Greenhouse job board's public listing.
package greenhouse

import (
	"cmp"
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/normalize"
)

const Base = "https://boards-api.greenhouse.io"

type Fetcher struct {
	board  boards.Board
	client *httpjson.Client
	base   string
}

func New(b boards.Board, c *httpjson.Client, base string) *Fetcher {
	return &Fetcher{board: b, client: c, base: base}
}

func (f *Fetcher) Name() string   { return f.board.Name() }
func (f *Fetcher) Rendered() bool { return false }

type response struct {
	Jobs []struct {
		ID             int64  `json:"id"`
		Title          string `json:"title"`
		AbsoluteURL    string `json:"absolute_url"`
		CompanyName    string `json:"company_name"`
		FirstPublished string `json:"first_published"`
		Content        string `json:"content"`
		Location       struct {
			Name string `json:"name"`
		} `json:"location"`
	} `json:"jobs"`
}

// Fetch reads the whole board in one call. content=true inlines each body,
// entity-escaped once, which Fetch unescapes into description_html.
func (f *Fetcher) Fetch(ctx context.Context) ([]normalize.Posting, error) {
	var r response
	url := fmt.Sprintf("%s/v1/boards/%s/jobs?content=true", f.base, f.board.Slug)
	if err := f.client.Get(ctx, url, &r); err != nil {
		return nil, fmt.Errorf("greenhouse %s: %w", f.board.Slug, err)
	}
	out := make([]normalize.Posting, 0, len(r.Jobs))
	for _, j := range r.Jobs {
		p := normalize.New(f.board.Source, f.board.Slug, strconv.FormatInt(j.ID, 10))
		p.Company = cmp.Or(j.CompanyName, f.board.Company)
		p.Title = strings.TrimSpace(j.Title)
		p.Location = j.Location.Name
		p.Remote = normalize.RemoteIn(j.Location.Name)
		p.URL = j.AbsoluteURL
		p.DescriptionHTML = html.UnescapeString(j.Content)
		p.DescriptionText = normalize.Text(p.DescriptionHTML)
		p.PostedAt = normalize.StampFrom(time.RFC3339, j.FirstPublished)
		out = append(out, p)
	}
	return out, nil
}
