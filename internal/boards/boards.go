// Package boards loads boards.yaml, the hand-curated list that is the crawl's
// whole coverage. Covering another board is adding an entry, never code.
package boards

import (
	"bytes"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"

	"github.com/tunedev/feed/internal/normalize"
)

// Board is one entry: which source serves it and the slug it has there.
// Company names boards whose API omits it; Region selects a Lever account's
// EU host; URL, Link and Empty describe a rendered page. Which of these a
// source requires is its fetcher's decision.
type Board struct {
	Source  string `yaml:"source"`
	Slug    string `yaml:"slug"`
	Company string `yaml:"company"`
	Region  string `yaml:"region"`
	URL     string `yaml:"url"`
	Link    string `yaml:"link"`
	Empty   string `yaml:"empty"`
}

// Name is the board's address under postings/ and status/.
func (b Board) Name() string { return b.Source + "/" + b.Slug }

type file struct {
	Boards []Board `yaml:"boards"`
}

// Load reads path and rejects a file that lists nothing, names an unknown
// field, or gives a source or slug that is not path-safe or repeats.
func Load(path string) ([]Board, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("boards: read %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var f file
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("boards: parse %s: %w", path, err)
	}
	if len(f.Boards) == 0 {
		return nil, fmt.Errorf("boards: %s lists no boards", path)
	}
	seen := make(map[string]bool, len(f.Boards))
	for _, b := range f.Boards {
		if !normalize.SafeSegment(b.Source) || !normalize.SafeSegment(b.Slug) {
			return nil, fmt.Errorf("boards: %q/%q is not a path-safe source and slug", b.Source, b.Slug)
		}
		if seen[b.Name()] {
			return nil, fmt.Errorf("boards: %s is listed twice", b.Name())
		}
		seen[b.Name()] = true
	}
	return f.Boards, nil
}
