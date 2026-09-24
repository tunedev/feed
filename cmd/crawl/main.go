// Command crawl fetches every board in boards.yaml and writes the feed's
// tree: postings/, status/ and runs/. It commits nothing; the workflow does.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/crawl"
	"github.com/tunedev/feed/internal/fetch/ashby"
	"github.com/tunedev/feed/internal/fetch/greenhouse"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/fetch/lever"
	"github.com/tunedev/feed/internal/fetch/remoteok"
	"github.com/tunedev/feed/internal/fetch/rendered"
	"github.com/tunedev/feed/internal/fetch/workable"
	"github.com/tunedev/feed/internal/fixtures"
	"github.com/tunedev/feed/internal/normalize"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "crawl: %v\n", err)
		os.Exit(1)
	}
}

type options struct {
	root, boards, userAgent                         string
	retention, timeout, crawlTimeout, renderTimeout time.Duration
	maxBytes                                        int64
	dryRun, render                                  bool
}

func parse(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("crawl", flag.ContinueOnError)
	fs.StringVar(&o.root, "root", ".", "feed checkout to write postings/, status/ and runs/ into")
	fs.StringVar(&o.boards, "boards", "boards.yaml", "board list")
	fs.StringVar(&o.userAgent, "user-agent", "tunedev-feed (+https://github.com/tunedev/feed)", "User-Agent sent to every board")
	fs.DurationVar(&o.retention, "retention", 90*24*time.Hour, "how long a removed posting stays in the tree")
	fs.DurationVar(&o.timeout, "timeout", 30*time.Second, "timeout for one board request")
	fs.DurationVar(&o.crawlTimeout, "crawl-timeout", 20*time.Minute, "bound on the whole crawl")
	fs.Int64Var(&o.maxBytes, "max-bytes", 32<<20, "max response size for one board, in bytes")
	fs.BoolVar(&o.dryRun, "dry-run", false, "crawl recorded fixtures into a new temporary directory")
	fs.DurationVar(&o.renderTimeout, "render-timeout", 45*time.Second, "how long a rendered page has to show its roles")
	fs.BoolVar(&o.render, "render", true, "drive the installed Chrome for rendered boards")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if o.retention <= 0 || o.timeout <= 0 || o.crawlTimeout <= 0 || o.maxBytes <= 0 || o.renderTimeout <= 0 {
		return options{}, errors.New("retention, timeouts and max-bytes must be positive")
	}
	return o, nil
}

func run(args []string, out io.Writer) error {
	o, err := parse(args)
	if err != nil {
		return err
	}
	list, err := boards.Load(o.boards)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: o.timeout}
	if o.dryRun {
		client.Transport = fixtures.Transport()
		if o.root, err = os.MkdirTemp("", "feed-dry-run-"); err != nil {
			return err
		}
		fmt.Fprintf(out, "dry run into %s\n", o.root)
		o.render = false
	}

	browser, closeBrowser, reason := openBrowser(o.render)
	defer closeBrowser()

	fetchers, err := build(list, factories(httpjson.New(client, o.maxBytes, o.userAgent), browser, reason, o.renderTimeout))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), o.crawlTimeout)
	defer cancel()
	m, err := crawl.Run(ctx, o.root, fetchers, time.Now, o.retention)
	if err != nil {
		return err
	}

	summary, err := normalize.Encode(m)
	if err != nil {
		return err
	}
	_, _ = out.Write(summary)
	if m.AllFailed() {
		return errors.New("every fetcher failed")
	}
	return nil
}

type factory func(boards.Board) (crawl.Fetcher, error)

// factories maps each source to its fetcher's constructor. Supporting a new
// source is one entry here and one package under internal/fetch. With no
// browser, rendered boards get a fetcher that reports why they cannot render.
func factories(get *httpjson.Client, browser *rod.Browser, reason string, renderTimeout time.Duration) map[string]factory {
	renderedBoard := func(b boards.Board) (crawl.Fetcher, error) { return rendered.NewUnrendered(b, reason), nil }
	if browser != nil {
		renderedBoard = func(b boards.Board) (crawl.Fetcher, error) { return rendered.New(b, browser, renderTimeout) }
	}
	return map[string]factory{
		"greenhouse": func(b boards.Board) (crawl.Fetcher, error) { return greenhouse.New(b, get, greenhouse.Base), nil },
		"lever":      func(b boards.Board) (crawl.Fetcher, error) { return lever.New(b, get, lever.Base, lever.EUBase) },
		"ashby":      func(b boards.Board) (crawl.Fetcher, error) { return ashby.New(b, get, ashby.Base) },
		"workable":   func(b boards.Board) (crawl.Fetcher, error) { return workable.New(b, get, workable.Base), nil },
		"remoteok":   func(b boards.Board) (crawl.Fetcher, error) { return remoteok.New(b, get, remoteok.Base), nil },
		"rendered":   renderedBoard,
	}
}

// openBrowser launches the installed Chrome, sandboxed. With rendering off,
// no Chrome installed, or a launch that fails, it returns a nil browser and
// the reason, and the crawl continues with every rendered board reporting
// needs_rendering for that reason.
func openBrowser(render bool) (*rod.Browser, func(), string) {
	noop := func() {}
	if !render {
		return nil, noop, "rendering disabled"
	}
	bin, found := launcher.LookPath()
	if !found {
		return nil, noop, "no browser installed"
	}
	u, err := launcher.New().Bin(bin).Headless(true).Launch()
	if err != nil {
		return nil, noop, "browser failed to launch: " + err.Error()
	}
	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		return nil, noop, "browser failed to connect: " + err.Error()
	}
	return b, func() { _ = b.Close() }, ""
}

// build turns each board into its fetcher. A board naming a source with no
// factory, or one its factory refuses, fails the run before anything is
// fetched.
func build(list []boards.Board, fs map[string]factory) ([]crawl.Fetcher, error) {
	out := make([]crawl.Fetcher, 0, len(list))
	for _, b := range list {
		newFetcher, ok := fs[b.Source]
		if !ok {
			return nil, fmt.Errorf("board %s: no fetcher for source %q", b.Name(), b.Source)
		}
		f, err := newFetcher(b)
		if err != nil {
			return nil, fmt.Errorf("board %s: %w", b.Name(), err)
		}
		out = append(out, f)
	}
	return out, nil
}
