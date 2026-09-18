package typesafe

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

type brokenBody struct{ closed bool }

func (*brokenBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (b *brokenBody) Close() error {
	b.closed = true
	return nil
}

func TestInterruptedResponseIsClosedAndRetried(t *testing.T) {
	body := &brokenBody{}
	calls := 0
	policy := DefaultRetryPolicy()
	policy.InitialDelay, policy.MaxDelay = 0, 0
	client := retryClient(t, func(request *http.Request) (*http.Response, error) {
		calls++
		response := mockResponse(200, nil)
		if calls == 1 {
			response.Body = body
		}
		return response, nil
	}, policy)
	if _, err := client.ListModels(t.Context()); err != nil || calls != 2 || !body.closed {
		t.Fatalf("calls=%d closed=%v err=%v", calls, body.closed, err)
	}
}

type delayedBody struct{ ctx context.Context }

func (b delayedBody) Read([]byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (delayedBody) Close() error { return nil }

func TestTimeoutIncludesResponseBody(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := retryClient(t, func(request *http.Request) (*http.Response, error) {
			response := mockResponse(200, nil)
			response.Body = delayedBody{ctx: request.Context()}
			return response, nil
		}, RetryPolicy{})
		start := time.Now()
		if _, err := client.ListModels(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("body read was not canceled: %v", err)
		}
		if time.Since(start) != 10*time.Second {
			t.Fatalf("attempt timeout was not applied: %s", time.Since(start))
		}
	})
}

func TestResponseBodyLimitDoesNotRetry(t *testing.T) {
	calls := 0
	client := retryClient(t, func(request *http.Request) (*http.Response, error) {
		calls++
		response := mockResponse(200, http.Header{"X-Request-Id": {"req_large"}})
		response.Body = io.NopCloser(strings.NewReader(strings.Repeat(" ", maxResponseBytes+1)))
		return response, nil
	}, DefaultRetryPolicy())
	_, err := client.ListModels(t.Context())
	responseErr, ok := errors.AsType[*ResponseError](err)
	if !ok || responseErr.RequestID != "req_large" || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestInvalidRequestsNeverUseTransport(t *testing.T) {
	client := retryClient(t, func(request *http.Request) (*http.Response, error) {
		t.Error("invalid request reached the transport")
		return mockResponse(200, nil), nil
	}, RetryPolicy{})
	if _, err := client.SystemOne(t.Context(), SystemOneRequest{}); err == nil {
		t.Fatal("empty questions accepted")
	}
	if _, err := client.ListModels(nil); err == nil {
		t.Fatal("nil context accepted")
	}
	var zero Client
	if _, err := zero.ListModels(t.Context()); err == nil {
		t.Fatal("zero client accepted")
	}
	var absent *Client
	if _, err := absent.SystemOne(t.Context(), testRequest()); err == nil {
		t.Fatal("nil client accepted")
	}
}

func TestCustomClientIsNotMutated(t *testing.T) {
	sentinel := errors.New("caller redirect policy")
	custom := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return sentinel }}
	client, err := NewClient(WithAPIKey("key"), WithHTTPClient(custom))
	if err != nil {
		t.Fatal(err)
	}
	if client.config.httpClient == custom || custom.CheckRedirect(nil, nil) != sentinel {
		t.Fatal("caller client was changed")
	}
}

func TestRetryAfterDatesAndJitter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		policy := DefaultRetryPolicy()
		future := time.Now().Add(8 * time.Second).UTC().Format(http.TimeFormat)
		if got := retryDelay(policy, 0, http.Header{"Retry-After": {future}}); got != 8*time.Second {
			t.Fatalf("date delay=%s", got)
		}
		for range 100 {
			if got := retryDelay(policy, 0, nil); got < 375*time.Millisecond || got > 500*time.Millisecond {
				t.Fatalf("jitter delay out of bounds: %s", got)
			}
		}
	})
}
