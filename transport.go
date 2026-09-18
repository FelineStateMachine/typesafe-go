package typesafe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strconv"
)

const maxResponseBytes = 16 << 20

type wireResponse struct {
	status int
	header http.Header
	body   []byte
}

func (r *wireResponse) requestID() string {
	for _, name := range []string{"x-request-id", "request-id", "x-typesafe-request-id"} {
		if value := r.header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

type transportRequest struct {
	method string
	url    string
	body   []byte
}

type transportError struct {
	err     error
	timeout bool
}

func (e *transportError) Error() string { return "typesafe: HTTP transport failed: " + e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

func (c *Client) send(ctx context.Context, method, path string, body []byte) (*wireResponse, error) {
	request := transportRequest{method: method, url: c.config.baseURL + path, body: body}
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		response, err := c.attempt(ctx, request, attempt)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var headers http.Header
		if err == nil {
			if response.status >= 200 && response.status < 300 {
				return response, nil
			}
			err = response.apiError(request)
			if !retryableStatus(response.status) {
				return nil, err
			}
			headers = response.header
		} else if !c.config.retry.retriesError(err) {
			return nil, err
		}
		if attempt >= c.config.retry.MaxRetries {
			return nil, err
		}
		if err := waitForRetry(ctx, retryDelay(c.config.retry, attempt, headers)); err != nil {
			return nil, err
		}
	}
}

func (r *wireResponse) apiError(request transportRequest) *APIError {
	return &APIError{
		StatusCode: r.status, RequestID: r.requestID(), Header: r.header,
		Body: r.body, Method: request.method, URL: request.url,
		Message: http.StatusText(r.status),
	}
}

func (c *Client) attempt(ctx context.Context, spec transportRequest, attempt int) (*wireResponse, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, c.config.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(attemptCtx, spec.method, spec.url, bytes.NewReader(spec.body))
	if err != nil {
		return nil, fmt.Errorf("typesafe: construct HTTP request: %w", err)
	}
	request.Header = c.requestHeaders(spec.body != nil, attempt)
	response, err := c.config.httpClient.Do(request)
	if err != nil {
		return nil, wrapTransportError(attemptCtx, err)
	}
	wire, err := readResponse(response)
	if err != nil {
		if _, ok := errors.AsType[*ResponseError](err); ok {
			return nil, err
		}
		return nil, wrapTransportError(attemptCtx, err)
	}
	return wire, nil
}

func (c *Client) requestHeaders(hasBody bool, attempt int) http.Header {
	headers := c.config.headers.Clone()
	headers.Set("Authorization", "Bearer "+c.config.apiKey)
	headers.Set("Accept", "application/json")
	headers.Set("User-Agent", "typesafe-go/"+Version)
	headers.Set("X-TypeSafe-SDK", "typesafe-go/"+Version)
	headers.Set("X-TypeSafe-Runtime", runtime.Version()+" "+runtime.GOOS+"/"+runtime.GOARCH)
	headers.Del("Content-Type")
	if hasBody {
		headers.Set("Content-Type", "application/json")
	}
	headers.Del("X-TypeSafe-Retry-Count")
	if attempt > 0 {
		headers.Set("X-TypeSafe-Retry-Count", strconv.Itoa(attempt))
	}
	return headers
}

func readResponse(response *http.Response) (*wireResponse, error) {
	wire := &wireResponse{status: response.StatusCode, header: response.Header.Clone()}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	closeErr := response.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	if len(body) > maxResponseBytes {
		return nil, &ResponseError{RequestID: wire.requestID(), Err: fmt.Errorf("response exceeds 16 MiB limit")}
	}
	wire.body = body
	return wire, nil
}

func wrapTransportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	timeout := errors.Is(err, context.DeadlineExceeded)
	if network, ok := errors.AsType[net.Error](err); ok {
		timeout = timeout || network.Timeout()
	}
	return &transportError{err: err, timeout: timeout}
}
