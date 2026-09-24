package remoteok_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/fetch/remoteok"
	"github.com/tunedev/feed/internal/fixtures"
)

// cloudflareLike refuses a request without a descriptive User-Agent, as
// RemoteOK's edge does.
func cloudflareLike(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.UserAgent()
		if ua == "" || strings.HasPrefix(ua, "Go-http-client") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.URL.Path != "/api" {
			t.Errorf("requested %s", r.URL.Path)
		}
		_, _ = w.Write(fixtures.Read("remoteok.json"))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

var board = boards.Board{Source: "remoteok", Slug: "all"}

func TestFetchSkipsTheLegalNoticeAndMapsJobs(t *testing.T) {
	c := httpjson.New(&http.Client{Timeout: 5 * time.Second}, 1<<20, "feed-test (+https://example.com)")
	got, err := remoteok.New(board, c, cloudflareLike(t)).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d postings, want 2: the legal notice is not a job", len(got))
	}
	p := got[0]
	checks := map[string][2]string{
		"id":               {p.ID, "remoteok/all/1137427"},
		"company":          {p.Company, "Prenosis"},
		"title":            {p.Title, "Software Engineer"},
		"url":              {p.URL, "https://remoteOK.com/remote-jobs/remote-software-engineer-prenosis-1137427"},
		"description_text": {p.DescriptionText, "Prenosis is an artificial intelligence company."},
		"posted_at":        {p.PostedAt, "2026-09-23T14:00:02Z"},
	}
	for field, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", field, c[0], c[1])
		}
	}
	if !p.Remote {
		t.Error("every RemoteOK posting is remote")
	}
}

func TestFetchFailsWithoutADescriptiveUserAgent(t *testing.T) {
	c := httpjson.New(&http.Client{Timeout: 5 * time.Second}, 1<<20, "")
	if _, err := remoteok.New(board, c, cloudflareLike(t)).Fetch(context.Background()); err == nil {
		t.Error("an anonymous request succeeded; the fixture server should have refused it")
	}
}
