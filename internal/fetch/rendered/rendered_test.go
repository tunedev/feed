package rendered_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

	"github.com/tunedev/feed/internal/boards"
	"github.com/tunedev/feed/internal/crawl"
	"github.com/tunedev/feed/internal/fetch/rendered"
)

func browser(t *testing.T) *rod.Browser {
	t.Helper()
	bin, found := launcher.LookPath()
	if !found {
		t.Skip("no Chrome installed; rendered fetches run where one is, as on CI runners")
	}
	u, err := launcher.New().Bin(bin).Headless(true).Launch()
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func serve(t *testing.T, page string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func board(url string) boards.Board {
	return boards.Board{Source: "rendered", Slug: "acme", Company: "Acme", URL: url, Link: "a.role", Empty: ".no-roles"}
}

// lateRoles holds no role anchor until its script runs: an HTTP-only fetch of
// it sees none, which is exactly the silent zero rendering exists to prevent.
const lateRoles = `<html><body><div id="roles"></div><script>
setTimeout(function () {
  var list = document.getElementById("roles");
  [["r-1", "Platform Engineer\nRemote"], ["r-2", "Designer"]].forEach(function (r) {
    var a = document.createElement("a");
    a.className = "role";
    a.href = "/jobs/" + r[0];
    a.innerText = r[1];
    list.appendChild(a);
  });
}, 300);
</script></body></html>`

func TestFetchWaitsForRolesThatRenderLate(t *testing.T) {
	if strings.Contains(lateRoles, `class="role"`) {
		t.Fatal("fixture defect: the static HTML already holds a role anchor")
	}
	url := serve(t, lateRoles)
	f, err := rendered.New(board(url), browser(t), 10*time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d postings, want 2", len(got))
	}
	p := got[0]
	if p.ID != "rendered/acme/r-1" || p.Title != "Platform Engineer" || p.URL != url+"/jobs/r-1" {
		t.Errorf("posting = %+v", p)
	}
	if !p.Remote || p.Company != "Acme" || p.RenderStatus != "rendered" {
		t.Errorf("posting = %+v", p)
	}
	if err := p.Validate(); err != nil {
		t.Errorf("not writable: %v", err)
	}
}

func TestFetchTreatsTheEmptyMarkerAsAGenuineZero(t *testing.T) {
	f, err := rendered.New(board(serve(t, `<div class="no-roles">No openings</div>`)), browser(t), 10*time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := f.Fetch(context.Background())
	if err != nil || len(got) != 0 {
		t.Errorf("got %d postings, err %v; want a genuine zero", len(got), err)
	}
}

func TestFetchReportsNeedsRenderingWhenNothingAppears(t *testing.T) {
	f, err := rendered.New(board(serve(t, `<div>loading</div>`)), browser(t), time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = f.Fetch(context.Background())
	if !errors.Is(err, crawl.ErrNeedsRendering) {
		t.Errorf("err = %v, want ErrNeedsRendering: a page that shows nothing is not an empty board", err)
	}
}

func TestUnrenderedNeverGuesses(t *testing.T) {
	u := rendered.NewUnrendered(board("https://example.com"), "no browser installed")
	got, err := u.Fetch(context.Background())
	if !errors.Is(err, crawl.ErrNeedsRendering) || got != nil || !u.Rendered() {
		t.Errorf("Unrendered = %v, %v", got, err)
	}
	if !strings.Contains(err.Error(), "no browser installed") {
		t.Errorf("reason lost: %v", err)
	}
}

func TestNewRequiresURLLinkAndCompany(t *testing.T) {
	for _, drop := range []string{"url", "link", "company"} {
		b := board("https://example.com")
		switch drop {
		case "url":
			b.URL = ""
		case "link":
			b.Link = ""
		case "company":
			b.Company = ""
		}
		if _, err := rendered.New(b, nil, time.Second); err == nil {
			t.Errorf("New accepted a board with no %s", drop)
		}
	}
}
