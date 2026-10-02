package normalize_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/normalize"
)

func valid(source, board, ext string) normalize.Posting {
	p := normalize.New(source, board, ext)
	p.Title = "Engineer"
	p.URL = "https://example.com/" + ext
	return p
}

func TestNewAddressesByPathAndStartsOpenAndStatic(t *testing.T) {
	p := normalize.New("lever", "acme", "abc-123")
	if p.ID != "lever/acme/abc-123" {
		t.Errorf("ID = %q, want lever/acme/abc-123", p.ID)
	}
	if p.SchemaVersion != 1 || p.Status != "open" || p.RenderStatus != "static" {
		t.Errorf("got version %d, status %q, render %q; want 1, open, static", p.SchemaVersion, p.Status, p.RenderStatus)
	}
}

func TestValidateAcceptsAWellFormedPosting(t *testing.T) {
	if err := valid("lever", "acme", "abc-123").Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejectsUnsafeIDs(t *testing.T) {
	for _, ext := range []string{"..", ".", ".hidden", "a/b", "", `a\b`, "a b"} {
		if err := valid("lever", "acme", ext).Validate(); err == nil {
			t.Errorf("external id %q accepted; it could escape postings/ or collide", ext)
		}
	}
}

func TestValidateRejectsAnIDThatDisagreesWithSourceAndBoard(t *testing.T) {
	p := valid("lever", "acme", "x")
	p.Board = "other"
	if err := p.Validate(); err == nil {
		t.Error("an id under lever/acme was accepted for board other")
	}
}

func TestValidateRequiresTitleAndURL(t *testing.T) {
	noTitle := valid("lever", "acme", "x")
	noTitle.Title = ""
	noURL := valid("lever", "acme", "x")
	noURL.URL = ""
	for name, p := range map[string]normalize.Posting{"title": noTitle, "url": noURL} {
		if err := p.Validate(); err == nil {
			t.Errorf("posting with no %s accepted", name)
		}
	}
}

func TestStampFromRestampsInUTCAndDropsTheUnparseable(t *testing.T) {
	if got := normalize.StampFrom(time.RFC3339, "2026-05-22T09:16:29-04:00"); got != "2026-05-22T13:16:29Z" {
		t.Errorf("StampFrom = %q, want 2026-05-22T13:16:29Z", got)
	}
	if got := normalize.StampFrom(time.RFC3339, "2021-04-27T20:13:45.158+00:00"); got != "2021-04-27T20:13:45Z" {
		t.Errorf("fractional seconds: StampFrom = %q, want 2021-04-27T20:13:45Z", got)
	}
	if got := normalize.StampFrom(time.RFC3339, "not-a-date"); got != "" {
		t.Errorf("StampFrom of garbage = %q, want empty", got)
	}
}

func TestRemoteInIsCaseInsensitive(t *testing.T) {
	if !normalize.RemoteIn("REMOTE, Bangalore") || normalize.RemoteIn("San Francisco") {
		t.Error("RemoteIn misread a location")
	}
}

func TestEncodeKeepsHTMLReadableAndOmitsEmptyOptionals(t *testing.T) {
	p := valid("lever", "acme", "x")
	p.DescriptionHTML = "<p>a & b</p>"
	p.FetchedAt = "2026-09-24T06:00:00Z"
	b, err := normalize.Encode(p)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"description_html": "<p>a & b</p>"`) {
		t.Errorf("HTML was escaped or the field is missing:\n%s", s)
	}
	for _, absent := range []string{"removed_at", "posted_at"} {
		if strings.Contains(s, absent) {
			t.Errorf("empty %s was written:\n%s", absent, s)
		}
	}
	if !strings.HasSuffix(s, "}\n") {
		t.Error("encoded document does not end in a newline")
	}
}
