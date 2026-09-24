// Package normalize defines the one document shape every source is mapped
// into, and the helpers fetchers share to produce it.
package normalize

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SchemaVersion is the shape of a Posting document. It increments only on a
// breaking change; see schema/v1.md.
const SchemaVersion = 1

const (
	StatusOpen    = "open"
	StatusRemoved = "removed"

	RenderStatic         = "static"
	RenderRendered       = "rendered"
	RenderNeedsRendering = "needs_rendering"
	RenderError          = "error"
)

// Posting is one normalised posting: one file under postings/. Times are
// RFC 3339 strings in UTC, empty when unknown, so an unknown time is left out
// of the document rather than written as a zero date.
type Posting struct {
	SchemaVersion   int    `json:"schema_version"`
	ID              string `json:"id"`
	Source          string `json:"source"`
	Board           string `json:"board"`
	Company         string `json:"company"`
	Title           string `json:"title"`
	Location        string `json:"location"`
	Remote          bool   `json:"remote"`
	URL             string `json:"url"`
	DescriptionText string `json:"description_text"`
	DescriptionHTML string `json:"description_html,omitempty"`
	PostedAt        string `json:"posted_at,omitempty"`
	FetchedAt       string `json:"fetched_at"`
	Status          string `json:"status"`
	RemovedAt       string `json:"removed_at,omitempty"`
	RenderStatus    string `json:"render_status"`
}

// New starts a Posting addressed by source, board and the board's own id. It
// is open and statically fetched until the caller says otherwise.
func New(source, board, externalID string) Posting {
	return Posting{
		SchemaVersion: SchemaVersion,
		ID:            source + "/" + board + "/" + externalID,
		Source:        source,
		Board:         board,
		Status:        StatusOpen,
		RenderStatus:  RenderStatic,
	}
}

var segment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// SafeSegment reports whether s can be one segment of an id, and so one
// component of a file path: non-empty, not starting with a dot, and free of
// separators and spaces.
func SafeSegment(s string) bool { return segment.MatchString(s) }

// Validate rejects a Posting that cannot be written: an id that is not three
// path-safe segments agreeing with Source and Board, or a missing title or url.
func (p Posting) Validate() error {
	parts := strings.Split(p.ID, "/")
	if len(parts) != 3 {
		return fmt.Errorf("posting %q: id must be source/board/external_id", p.ID)
	}
	for _, s := range parts {
		if !SafeSegment(s) {
			return fmt.Errorf("posting %q: id segment %q is not path-safe", p.ID, s)
		}
	}
	if parts[0] != p.Source || parts[1] != p.Board {
		return fmt.Errorf("posting %q: id disagrees with source %q and board %q", p.ID, p.Source, p.Board)
	}
	if p.Title == "" {
		return fmt.Errorf("posting %q: no title", p.ID)
	}
	if p.URL == "" {
		return fmt.Errorf("posting %q: no url", p.ID)
	}
	return nil
}

// Stamp formats t as the RFC 3339 UTC string every time field uses.
func Stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// StampFrom parses value with layout and restamps it in UTC. An unparseable
// value yields "", since every board-supplied date is optional.
func StampFrom(layout, value string) string {
	t, err := time.Parse(layout, value)
	if err != nil {
		return ""
	}
	return Stamp(t)
}

// RemoteIn reports whether a free-text location mentions remote work.
func RemoteIn(location string) bool {
	return strings.Contains(strings.ToLower(location), "remote")
}
