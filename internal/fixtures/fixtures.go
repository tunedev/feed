// Package fixtures holds recorded board responses, shaped like the real
// APIs', for offline tests and the crawl's dry run.
package fixtures

import "embed"

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
