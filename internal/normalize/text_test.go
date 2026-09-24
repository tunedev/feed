package normalize_test

import (
	"testing"

	"github.com/tunedev/feed/internal/normalize"
)

func TestTextDropsTagsDecodesEntitiesAndBreaksAtBlocks(t *testing.T) {
	got := normalize.Text("<p>Build &amp; ship.</p><ul><li>Go</li><li>SQL</li></ul>")
	if want := "Build & ship.\nGo\nSQL"; got != want {
		t.Errorf("Text = %q, want %q", got, want)
	}
}

func TestTextCollapsesSourceWhitespace(t *testing.T) {
	if got := normalize.Text("<div>  a \n\t b </div>"); got != "a b" {
		t.Errorf("Text = %q, want %q", got, "a b")
	}
}
