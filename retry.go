package typesafe

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy controls retries after transient failures. All delays must be
// nonnegative, MaxDelay must be at least InitialDelay, and Jitter is in [0, 1].
// Retryable HTTP statuses are 408, 429, and 500–599. A caller's cancellation or
// deadline always stops the operation, regardless of this policy.
type RetryPolicy struct {
	MaxRetries            int
	InitialDelay          time.Duration
	MaxDelay              time.Duration
	MaxRetryAfter         time.Duration
	Jitter                float64
	RetryConnectionErrors bool
	RetryTimeouts         bool
}

// DefaultRetryPolicy returns an independent copy of the default settings:
// two retries, 500ms initial backoff, 5s maximum backoff, up to 25% downward
// jitter, and server-requested delays up to 60s. Network failures and attempt
// timeouts are retried. No total time budget is imposed; use a context deadline.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 2, InitialDelay: 500 * time.Millisecond,
		MaxDelay: 5 * time.Second, MaxRetryAfter: 60 * time.Second,
		Jitter: 0.25, RetryConnectionErrors: true, RetryTimeouts: true,
	}
}

func (p RetryPolicy) validate() error {
	if p.MaxRetries < 0 || p.InitialDelay < 0 || p.MaxDelay < p.InitialDelay || p.MaxRetryAfter < 0 {
		return fmt.Errorf("typesafe: invalid retry count or delays")
	}
	if math.IsNaN(p.Jitter) || p.Jitter < 0 || p.Jitter > 1 {
		return fmt.Errorf("typesafe: retry jitter must be between 0 and 1")
	}
	return nil
}

func retryableStatus(status int) bool {
	return status == 408 || status == 429 || (status >= 500 && status <= 599)
}

func (p RetryPolicy) retriesError(err error) bool {
	if _, ok := errors.AsType[*ResponseError](err); ok {
		return false
	}
	if transport, ok := errors.AsType[*transportError](err); ok {
		if transport.timeout {
			return p.RetryTimeouts
		}
		return p.RetryConnectionErrors
	}
	return false
}

func retryDelay(policy RetryPolicy, attempt int, headers http.Header) time.Duration {
	if delay, ok := serverDelay(headers, policy.MaxRetryAfter); ok {
		return delay
	}
	delay := policy.InitialDelay
	for range attempt {
		if delay == 0 {
			break
		}
		if delay >= policy.MaxDelay-delay {
			delay = policy.MaxDelay
			break
		}
		delay *= 2
	}
	return delay - time.Duration(float64(delay)*rand.Float64()*policy.Jitter)
}

func serverDelay(headers http.Header, maximum time.Duration) (time.Duration, bool) {
	if raw := headers.Get("retry-after-ms"); raw != "" {
		if delay, ok := numericDelay(raw, time.Millisecond, maximum); ok {
			return delay, true
		}
	}
	raw := headers.Get("Retry-After")
	if delay, ok := numericDelay(raw, time.Second, maximum); ok {
		return delay, true
	}
	date, err := http.ParseTime(raw)
	if err != nil {
		return 0, false
	}
	delay := max(time.Until(date), 0)
	return delay, delay <= maximum
}

func numericDelay(raw string, unit, maximum time.Duration) (time.Duration, bool) {
	number, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
		return 0, false
	}
	nanoseconds := number * float64(unit)
	if nanoseconds >= float64(math.MaxInt64) || nanoseconds > float64(maximum) {
		return 0, false
	}
	return time.Duration(nanoseconds), true
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}
