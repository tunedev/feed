package normalize

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Encode renders v as the indented JSON every file in the feed uses, with
// HTML left unescaped so description_html stays readable in a diff.
func Encode(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("normalize: encode: %w", err)
	}
	return b.Bytes(), nil
}
