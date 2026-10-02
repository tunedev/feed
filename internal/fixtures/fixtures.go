// Package fixtures holds recorded board responses, shaped like the real
// APIs', for offline tests and the crawl's dry run.
package fixtures

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"net/http"
)

//go:embed *.json
var files embed.FS

// Read returns the recorded response in name. It panics on a missing name:
// that is a broken test, not a runtime condition.
func Read(name string) []byte {
	b, err := files.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return b
}

// byHost names the recorded response each board API's host answers with.
var byHost = map[string]string{
	"boards-api.greenhouse.io": "greenhouse.json",
	"api.lever.co":             "lever.json",
	"api.eu.lever.co":          "lever.json",
	"api.ashbyhq.com":          "ashby.json",
	"apply.workable.com":       "workable.json",
	"remoteok.com":             "remoteok.json",
}

// Transport answers each request from the recorded response for its host,
// so a dry run exercises every fetcher's real request and mapping with no
// network. A host with no recording fails the request.
func Transport() http.RoundTripper { return transport{} }

type transport struct{}

func (transport) RoundTrip(r *http.Request) (*http.Response, error) {
	name, ok := byHost[r.URL.Host]
	if !ok {
		return nil, fmt.Errorf("fixtures: no recorded response for host %s", r.URL.Host)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(Read(name))),
		Request:    r,
	}, nil
}
