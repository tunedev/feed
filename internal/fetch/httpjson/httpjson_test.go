package httpjson_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tunedev/feed/internal/fetch/httpjson"
)

func serve(t *testing.T, status int, body string, seenUA *string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seenUA != nil {
			*seenUA = r.UserAgent()
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func client(maxBytes int64) *httpjson.Client {
	return httpjson.New(&http.Client{Timeout: 5 * time.Second}, maxBytes, "feed-test (+https://example.com)")
}

func TestGetSendsTheUserAgentAndDecodes(t *testing.T) {
	var ua string
	url := serve(t, 200, `{"a":"12"}`, &ua)
	var v struct{ A string }
	if err := client(1<<20).Get(context.Background(), url, &v); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.A != "12" || ua != "feed-test (+https://example.com)" {
		t.Errorf("decoded %q with user agent %q", v.A, ua)
	}
}

func TestGetAcceptsABodyExactlyAtTheLimitAndRejectsOneOver(t *testing.T) {
	var v map[string]string
	if err := client(10).Get(context.Background(), serve(t, 200, `{"a":"12"}`, nil), &v); err != nil {
		t.Errorf("10-byte body at a 10-byte limit: %v", err)
	}
	if err := client(10).Get(context.Background(), serve(t, 200, `{"a":"123"}`, nil), &v); err == nil {
		t.Error("11-byte body at a 10-byte limit was accepted")
	}
}

func TestGetFailsOnANon2xxStatus(t *testing.T) {
	var v map[string]string
	if err := client(1<<20).Get(context.Background(), serve(t, 503, `{}`, nil), &v); err == nil {
		t.Error("a 503 was decoded as success")
	}
}
