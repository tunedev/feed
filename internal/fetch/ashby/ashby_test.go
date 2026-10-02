package ashby_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/ashby"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/fixtures"
)

var board = boards.Board{Source: "ashby", Slug: "linear", Company: "Linear"}

func TestFetchMapsListedJobsAndSkipsUnlisted(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write(fixtures.Read("ashby.json"))
	}))
	defer srv.Close()

	f, err := ashby.New(board, httpjson.New(&http.Client{Timeout: 5 * time.Second}, 1<<20, "feed-test"), srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if path != "/posting-api/job-board/linear?includeCompensation=true" {
		t.Errorf("requested %s", path)
	}
	if len(got) != 1 {
		t.Fatalf("got %d postings, want 1: the unlisted job must be skipped", len(got))
	}
	p := got[0]
	checks := map[string][2]string{
		"id":               {p.ID, "ashby/linear/d3bc1ced-3ce4-4086-a050-555055dbb1ff"},
		"company":          {p.Company, "Linear"},
		"title":            {p.Title, "Senior / Staff Fullstack Engineer"},
		"location":         {p.Location, "Europe"},
		"url":              {p.URL, "https://jobs.ashbyhq.com/linear/d3bc1ced-3ce4-4086-a050-555055dbb1ff"},
		"description_text": {p.DescriptionText, "Build Linear."},
		"posted_at":        {p.PostedAt, "2021-04-27T20:13:45Z"},
	}
	for field, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", field, c[0], c[1])
		}
	}
	if !p.Remote {
		t.Error("isRemote was not mapped")
	}
}

func TestNewRequiresACompany(t *testing.T) {
	if _, err := ashby.New(boards.Board{Source: "ashby", Slug: "linear"}, nil, "http://x.invalid"); err == nil {
		t.Error("New accepted an Ashby board with no company; the API does not supply one")
	}
}
