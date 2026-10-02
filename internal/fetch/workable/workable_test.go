package workable_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/fetch/httpjson"
	"github.com/tunedev/feed/internal/fetch/workable"
	"github.com/tunedev/feed/internal/fixtures"
)

// Without details=true the real list call omits every description; the
// server here refuses such a request so the quirk is pinned.
func TestFetchAsksForDetailsAndMapsThem(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if r.URL.Query().Get("details") != "true" {
			t.Errorf("request without details=true: %s", r.URL)
		}
		_, _ = w.Write(fixtures.Read("workable.json"))
	}))
	defer srv.Close()

	c := httpjson.New(&http.Client{Timeout: 5 * time.Second}, 1<<20, "feed-test")
	got, err := workable.New(boards.Board{Source: "workable", Slug: "huggingface"}, c, srv.URL).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if path != "/api/v1/widget/accounts/huggingface" {
		t.Errorf("requested %s", path)
	}
	if len(got) != 1 {
		t.Fatalf("got %d postings, want 1", len(got))
	}
	p := got[0]
	checks := map[string][2]string{
		"id":               {p.ID, "workable/huggingface/F4C096B22E"},
		"company":          {p.Company, "Hugging Face"},
		"location":         {p.Location, "Paris, Île-de-France, France"},
		"url":              {p.URL, "https://apply.workable.com/j/F4C096B22E"},
		"description_text": {p.DescriptionText, "At Hugging Face, we build."},
		"posted_at":        {p.PostedAt, "2026-07-30T00:00:00Z"},
	}
	for field, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", field, c[0], c[1])
		}
	}
	if !p.Remote {
		t.Error("telecommuting was not mapped to remote")
	}
}
