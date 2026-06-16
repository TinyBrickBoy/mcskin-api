package mojang

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// rtFunc adapts a function to an http.RoundTripper.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// egressClient builds an *http.Client whose transport always returns status.
func egressClient(status int, body string) *http.Client {
	return &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
}

func TestNewCountsProxies(t *testing.T) {
	c := New(time.Minute, []string{"socks5://10.0.0.2:1080", "  ", "http://h:3128", "://bad"})
	// direct + the two parseable proxies (blank and "://bad" are skipped)
	if got := len(c.egress); got != 3 {
		t.Fatalf("egress count = %d, want 3", got)
	}
}

func TestGetUsesDirectFirst(t *testing.T) {
	c := New(time.Minute, nil)
	c.egress = []*http.Client{egressClient(http.StatusOK, "direct"), egressClient(http.StatusOK, "proxy")}
	body, status, err := c.get(context.Background(), "http://x")
	if err != nil || status != http.StatusOK || string(body) != "direct" {
		t.Fatalf("got %q status=%d err=%v, want direct/200", body, status, err)
	}
}

func TestGetRotatesPastRateLimit(t *testing.T) {
	c := New(time.Minute, nil)
	c.egress = []*http.Client{
		egressClient(http.StatusTooManyRequests, ""), // direct is rate-limited
		egressClient(http.StatusTooManyRequests, ""), // proxy 1 too
		egressClient(http.StatusOK, "via-proxy-2"),   // proxy 2 has budget
	}
	body, status, err := c.get(context.Background(), "http://x")
	if err != nil || status != http.StatusOK || string(body) != "via-proxy-2" {
		t.Fatalf("got %q status=%d err=%v, want via-proxy-2/200", body, status, err)
	}
}

func TestGetAllRateLimitedReturns429(t *testing.T) {
	c := New(time.Minute, nil)
	c.egress = []*http.Client{
		egressClient(http.StatusTooManyRequests, ""),
		egressClient(http.StatusTooManyRequests, ""),
	}
	_, status, err := c.get(context.Background(), "http://x")
	if err != nil || status != http.StatusTooManyRequests {
		t.Fatalf("status=%d err=%v, want 429/nil", status, err)
	}
}

func TestGetSkipsUnreachableEgress(t *testing.T) {
	boom := &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF // proxy down
	})}
	c := New(time.Minute, nil)
	c.egress = []*http.Client{boom, egressClient(http.StatusOK, "recovered")}
	body, status, err := c.get(context.Background(), "http://x")
	if err != nil || status != http.StatusOK || string(body) != "recovered" {
		t.Fatalf("got %q status=%d err=%v, want recovered/200", body, status, err)
	}
}
