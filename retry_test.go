package typesafe

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func mockResponse(status int, header http.Header) *http.Response {
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(`{"models":[]}`))}
}

func retryClient(t *testing.T, transport roundTripFunc, policy RetryPolicy) *Client {
	t.Helper()
	client, err := NewClient(WithAPIKey("key"), WithBaseURL("https://test.invalid"),
		WithHTTPClient(&http.Client{Transport: transport}), WithRetryPolicy(policy))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestRetryStatusAndCount(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 408, 422, 429, 500, 503, 529, 599} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			policy := DefaultRetryPolicy()
			policy.InitialDelay, policy.MaxDelay = 0, 0
			client := retryClient(t, func(request *http.Request) (*http.Response, error) {
				calls++
				if calls > 1 && request.Header.Get("X-TypeSafe-Retry-Count") == "" {
					t.Error("retry identity header missing")
				}
				return mockResponse(status, make(http.Header)), nil
			}, policy)
			_, err := client.ListModels(context.Background())
			if _, ok := errors.AsType[*APIError](err); !ok {
				t.Fatalf("expected APIError, got %v", err)
			}
			want := 1
			if status == 408 || status == 429 || status >= 500 {
				want = 3
			}
			if calls != want {
				t.Fatalf("attempts=%d, want %d", calls, want)
			}
		})
	}
}

func TestRetryAfterAndCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		client := retryClient(t, func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return mockResponse(429, http.Header{"Retry-After": {"3"}}), nil
			}
			return mockResponse(200, make(http.Header)), nil
		}, DefaultRetryPolicy())
		start := time.Now()
		if _, err := client.ListModels(t.Context()); err != nil {
			t.Fatal(err)
		}
		if calls != 2 || time.Since(start) != 3*time.Second {
			t.Fatalf("calls=%d delay=%s", calls, time.Since(start))
		}
		calls = 0
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if _, err := client.ListModels(ctx); !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
			t.Fatalf("backoff ignored cancellation: calls=%d err=%v", calls, err)
		}
	})
}

func TestConnectionRetries(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		calls := 0
		policy := DefaultRetryPolicy()
		policy.RetryConnectionErrors = enabled
		policy.InitialDelay, policy.MaxDelay = 0, 0
		client := retryClient(t, func(request *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				return nil, io.ErrUnexpectedEOF
			}
			return mockResponse(200, make(http.Header)), nil
		}, policy)
		_, err := client.ListModels(context.Background())
		if enabled && (err != nil || calls != 2) {
			t.Fatalf("expected recovery: calls=%d err=%v", calls, err)
		}
		if !enabled && (!errors.Is(err, io.ErrUnexpectedEOF) || calls != 1) {
			t.Fatalf("unexpected retry: calls=%d err=%v", calls, err)
		}
	}
}

func TestRetryTimeoutPolicy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		for _, enabled := range []bool{false, true} {
			calls := 0
			policy := DefaultRetryPolicy()
			policy.RetryTimeouts = enabled
			policy.InitialDelay, policy.MaxDelay = 0, 0
			client := retryClient(t, func(request *http.Request) (*http.Response, error) {
				calls++
				<-request.Context().Done()
				return nil, request.Context().Err()
			}, policy)
			_, err := client.ListModels(t.Context())
			want := 1
			if enabled {
				want = 3
			}
			if !errors.Is(err, context.DeadlineExceeded) || calls != want {
				t.Fatalf("calls=%d want=%d err=%v", calls, want, err)
			}
		}
	})
}

func TestRetryDelay(t *testing.T) {
	policy := DefaultRetryPolicy()
	policy.Jitter = 0
	tests := []struct {
		name    string
		headers http.Header
		attempt int
		want    time.Duration
	}{
		{"initial", nil, 0, 500 * time.Millisecond},
		{"exponential", nil, 2, 2 * time.Second},
		{"capped", nil, 10000, 5 * time.Second},
		{"milliseconds", http.Header{"Retry-After-Ms": {"1250"}, "Retry-After": {"10"}}, 0, 1250 * time.Millisecond},
		{"zero", http.Header{"Retry-After": {"0"}}, 0, 0},
		{"invalid", http.Header{"Retry-After": {"garbage"}}, 0, 500 * time.Millisecond},
		{"too large", http.Header{"Retry-After": {"9999999999999999999999"}}, 0, 500 * time.Millisecond},
		{"negative", http.Header{"Retry-After": {"-1"}}, 0, 500 * time.Millisecond},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := retryDelay(policy, test.attempt, test.headers); got != test.want {
				t.Fatalf("delay=%s want=%s", got, test.want)
			}
		})
	}
}

func TestRetryDelayDurationBoundaries(t *testing.T) {
	policy := RetryPolicy{InitialDelay: time.Duration(math.MaxInt64), MaxDelay: time.Duration(math.MaxInt64)}
	if got := retryDelay(policy, 0, nil); got != policy.MaxDelay {
		t.Fatalf("maximum duration overflowed: %s", got)
	}
	policy.InitialDelay, policy.MaxDelay = 0, time.Nanosecond
	if got := retryDelay(policy, 1, nil); got != 0 {
		t.Fatalf("zero backoff became positive: %s", got)
	}
}
