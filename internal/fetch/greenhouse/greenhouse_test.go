package greenhouse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/greenhouse"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/fixtures"
)

func client() *httpjson.Client {
	return httpjson.New(&http.Client{Timeout: 5 * time.Second}, 1<<20, "feed-test")
}

var board = boards.Board{Source: "greenhouse", Slug: "gitlab"}

func TestFetchMapsTheWholeBoardInOneCall(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write(fixtures.Read("greenhouse.json"))
	}))
	defer srv.Close()

	got, err := greenhouse.New(board, client(), srv.URL).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if path != "/v1/boards/gitlab/jobs" || query != "content=true" {
		t.Errorf("requested %s?%s", path, query)
	}
	if len(got) != 2 {
		t.Fatalf("got %d postings, want 2", len(got))
	}
	p := got[0]
	checks := map[string][2]string{
		"id":               {p.ID, "greenhouse/gitlab/8556658002"},
		"company":          {p.Company, "GitLab"},
		"title":            {p.Title, "AI Engineer"},
		"location":         {p.Location, "Remote, Bangalore"},
		"url":              {p.URL, "https://job-boards.greenhouse.io/gitlab/jobs/8556658002"},
		"description_html": {p.DescriptionHTML, "<p>Build &amp; ship.</p><ul><li>Go</li></ul>"},
		"description_text": {p.DescriptionText, "Build & ship.\nGo"},
		"posted_at":        {p.PostedAt, "2026-05-22T13:16:29Z"},
	}
	for field, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", field, c[0], c[1])
		}
	}
	if !p.Remote || got[1].Remote {
		t.Errorf("remote = %v, %v; want true, false", p.Remote, got[1].Remote)
	}
	if got[1].PostedAt != "" {
		t.Errorf("unparseable first_published became %q, want empty", got[1].PostedAt)
	}
	for _, p := range got {
		if err := p.Validate(); err != nil {
			t.Errorf("mapped posting is not writable: %v", err)
		}
	}
}

func TestFetchFailsNamingTheBoard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := greenhouse.New(board, client(), srv.URL).Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "greenhouse gitlab") {
		t.Errorf("err = %v, want one naming greenhouse gitlab", err)
	}
}
