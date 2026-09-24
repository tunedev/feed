package lever_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/fetch/lever"
	"github.com/tunedev/feed/internal/fixtures"
	"github.com/tunedev/feed/internal/normalize"
)

func client() *httpjson.Client {
	return httpjson.New(&http.Client{Timeout: 5 * time.Second}, 1<<20, "feed-test")
}

func recorded(t *testing.T, path *string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path != nil {
			*path = r.URL.Path + "?" + r.URL.RawQuery
		}
		_, _ = w.Write(fixtures.Read("lever.json"))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func refuse(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request to the wrong host: %s", r.URL)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

var board = boards.Board{Source: "lever", Slug: "palantir", Company: "Palantir"}

func TestFetchMapsListsIntoTheBody(t *testing.T) {
	var path string
	f, err := lever.New(board, client(), recorded(t, &path), refuse(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if path != "/v0/postings/palantir?mode=json" {
		t.Errorf("requested %s", path)
	}
	if len(got) != 2 {
		t.Fatalf("got %d postings, want 2", len(got))
	}
	p := got[0]
	wantHTML := "<div>Support the team.</div><h3>What you'll do</h3><ul><li>Plan</li><li>Coordinate</li></ul><div>Equal opportunity.</div>"
	checks := map[string][2]string{
		"id":               {p.ID, "lever/palantir/6ed76ce8-4156-4b60-b120-403538bd66cd"},
		"company":          {p.Company, "Palantir"},
		"title":            {p.Title, "Administrative Business Partner"},
		"location":         {p.Location, "Singapore, Singapore"},
		"url":              {p.URL, "https://jobs.lever.co/palantir/6ed76ce8-4156-4b60-b120-403538bd66cd"},
		"description_html": {p.DescriptionHTML, wantHTML},
		"description_text": {p.DescriptionText, "Support the team.\nWhat you'll do\nPlan\nCoordinate\nEqual opportunity."},
		"posted_at":        {p.PostedAt, normalize.Stamp(time.UnixMilli(1786469891368))},
	}
	for field, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", field, c[0], c[1])
		}
	}
	if p.Remote || !got[1].Remote {
		t.Errorf("remote = %v, %v; want false (hybrid), true (remote)", p.Remote, got[1].Remote)
	}
}

func TestFetchUsesTheEUHostForAnEUBoard(t *testing.T) {
	eu := board
	eu.Region = "eu"
	f, err := lever.New(eu, client(), refuse(t), recorded(t, nil))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, err := f.Fetch(context.Background()); err != nil || len(got) != 2 {
		t.Errorf("EU fetch = %d postings, %v", len(got), err)
	}
}

func TestNewRejectsABoardItCannotServe(t *testing.T) {
	noCompany := board
	noCompany.Company = ""
	badRegion := board
	badRegion.Region = "mars"
	for name, b := range map[string]boards.Board{"no company": noCompany, "unknown region": badRegion} {
		if _, err := lever.New(b, client(), "http://x.invalid", "http://y.invalid"); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
	}
}
