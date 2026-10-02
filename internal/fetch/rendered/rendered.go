// Package rendered fetches a careers page that lists its roles only once
// JavaScript runs, by driving the browser the crawl was given.
package rendered

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/crawl"
	"github.com/tunedev/feed/internal/normalize"
)

type Fetcher struct {
	board   boards.Board
	browser *rod.Browser
	timeout time.Duration
}

// New serves a board with a page url, a CSS selector matching each role's
// anchor, and a company name. Empty, a selector the page shows when it has
// no openings, is optional.
func New(b boards.Board, browser *rod.Browser, timeout time.Duration) (*Fetcher, error) {
	if b.URL == "" || b.Link == "" || b.Company == "" {
		return nil, fmt.Errorf("rendered %s: url, link and company are required", b.Slug)
	}
	return &Fetcher{board: b, browser: browser, timeout: timeout}, nil
}

func (f *Fetcher) Name() string   { return f.board.Name() }
func (f *Fetcher) Rendered() bool { return true }

// Fetch opens the page and races the role selector against the empty
// selector. Roles yield postings; the empty marker is a genuine zero; neither
// within the timeout means the roles never rendered, reported as
// crawl.ErrNeedsRendering and never as an empty board.
func (f *Fetcher) Fetch(ctx context.Context) ([]normalize.Posting, error) {
	page, err := f.browser.Context(ctx).Page(proto.TargetCreateTarget{URL: f.board.URL})
	if err != nil {
		return nil, fmt.Errorf("rendered %s: open page: %w", f.board.Slug, err)
	}
	defer func() { _ = page.Close() }()

	found := false
	race := page.Timeout(f.timeout).Race().
		Element(f.board.Link).Handle(func(*rod.Element) error { found = true; return nil })
	if f.board.Empty != "" {
		race = race.Element(f.board.Empty)
	}
	if _, err := race.Do(); errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("rendered %s: no roles within %s: %w", f.board.Slug, f.timeout, crawl.ErrNeedsRendering)
	} else if err != nil {
		return nil, fmt.Errorf("rendered %s: %w", f.board.Slug, err)
	}
	if !found {
		return nil, nil
	}
	return f.extract(page)
}

// extract turns each anchor matching the role selector into a posting: its
// first line of text is the title, its resolved href the url, and the href's
// last path segment the id when that segment is path-safe.
func (f *Fetcher) extract(page *rod.Page) ([]normalize.Posting, error) {
	anchors, err := page.Elements(f.board.Link)
	if err != nil {
		return nil, fmt.Errorf("rendered %s: read roles: %w", f.board.Slug, err)
	}
	out := make([]normalize.Posting, 0, len(anchors))
	for _, a := range anchors {
		href, err := a.Property("href")
		if err != nil {
			return nil, fmt.Errorf("rendered %s: read href: %w", f.board.Slug, err)
		}
		text, err := a.Text()
		if err != nil {
			return nil, fmt.Errorf("rendered %s: read text: %w", f.board.Slug, err)
		}
		p := normalize.New(f.board.Source, f.board.Slug, externalID(href.Str()))
		p.Company = f.board.Company
		p.Title = firstLine(text)
		p.Remote = normalize.RemoteIn(text)
		p.URL = href.Str()
		p.RenderStatus = normalize.RenderRendered
		out = append(out, p)
	}
	return out, nil
}

// externalID is the last path segment of href, or a short hash of href when
// that segment could not be a file name.
func externalID(href string) string {
	if u, err := url.Parse(href); err == nil {
		if seg := path.Base(u.Path); normalize.SafeSegment(seg) {
			return seg
		}
	}
	sum := sha256.Sum256([]byte(href))
	return hex.EncodeToString(sum[:8])
}

func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// Unrendered stands in for Fetcher when the crawl has no browser. It never
// guesses from static HTML: every page it is given needs rendering, for the
// reason it was built with.
type Unrendered struct {
	board  boards.Board
	reason string
}

func NewUnrendered(b boards.Board, reason string) *Unrendered {
	return &Unrendered{board: b, reason: reason}
}

func (u *Unrendered) Name() string   { return u.board.Name() }
func (u *Unrendered) Rendered() bool { return true }

func (u *Unrendered) Fetch(context.Context) ([]normalize.Posting, error) {
	return nil, fmt.Errorf("rendered %s: %s: %w", u.board.Slug, u.reason, crawl.ErrNeedsRendering)
}
