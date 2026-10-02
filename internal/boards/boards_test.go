package boards_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tunedev/feed/internal/boards"
)

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "boards.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadReadsEveryField(t *testing.T) {
	got, err := boards.Load(write(t, `
boards:
  - {source: greenhouse, slug: gitlab}
  - {source: lever, slug: acme, company: Acme, region: eu}
  - source: rendered
    slug: acme-careers
    company: Acme
    url: https://example.com/careers
    link: a.role
    empty: .no-roles
`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d boards, want 3", len(got))
	}
	if got[1].Name() != "lever/acme" || got[1].Company != "Acme" || got[1].Region != "eu" {
		t.Errorf("lever board = %+v", got[1])
	}
	if got[2].URL != "https://example.com/careers" || got[2].Link != "a.role" || got[2].Empty != ".no-roles" {
		t.Errorf("rendered board = %+v", got[2])
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	cases := map[string]string{
		"unsafe slug":   "boards:\n  - {source: lever, slug: ../etc}\n",
		"duplicate":     "boards:\n  - {source: lever, slug: a, company: A}\n  - {source: lever, slug: a, company: A}\n",
		"unknown field": "boards:\n  - {source: lever, slug: a, regoin: eu}\n",
		"no boards":     "boards: []\n",
		"empty file":    "",
	}
	for name, content := range cases {
		if _, err := boards.Load(write(t, content)); err == nil {
			t.Errorf("%s: Load accepted it", name)
		}
	}
}

func TestTheSeedFileLoads(t *testing.T) {
	got, err := boards.Load(filepath.Join("..", "..", "boards.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) < 5 {
		t.Errorf("seed lists %d boards, want at least the five sources", len(got))
	}
}
