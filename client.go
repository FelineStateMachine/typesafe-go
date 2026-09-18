package typesafe

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
)

// Client calls the TypeSafe hosted API. A client can be shared by concurrent
// goroutines. Caller-owned request values and custom transports must also be
// safe for their usage; do not mutate a request while it is being marshaled.
// Construct clients with NewClient; the zero value is not ready for use.
type Client struct {
	config clientConfig
}

// NewClient constructs a client using environment variables and options.
// Explicit options take precedence. An API key is required. Configuration is
// validated locally; construction does not make any network requests.
func NewClient(options ...Option) (*Client, error) {
	config := defaultConfig()
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("typesafe: option must not be nil")
		}
		if err := option(&config); err != nil {
			return nil, err
		}
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	client := *config.httpClient
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	config.httpClient = &client
	return &Client{config: config}, nil
}

// SystemOne evaluates named questions against a shared state. Questions are
// validated before sending. A blank model uses the client's configured model.
// The context bounds the entire operation, including retries and backoff.
func (c *Client) SystemOne(ctx context.Context, request SystemOneRequest) (*SystemOneResponse, error) {
	if err := c.ready(ctx); err != nil {
		return nil, err
	}
	if request.Model == "" {
		request.Model = c.config.model
	}
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("typesafe: invalid request: %w", err)
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("typesafe: encode request: %w", err)
	}
	wire, err := c.send(ctx, http.MethodPost, "/v1/systemone", body)
	if err != nil {
		return nil, err
	}
	var result SystemOneResponse
	if err := json.Unmarshal(wire.body, &result); err != nil {
		return nil, &ResponseError{RequestID: wire.requestID(), Err: err}
	}
	result.RequestID = wire.requestID()
	return &result, nil
}

// ListModels lists models available to the authenticated account.
// An empty Models slice is a valid response.
func (c *Client) ListModels(ctx context.Context) (*ListModelsResponse, error) {
	if err := c.ready(ctx); err != nil {
		return nil, err
	}
	wire, err := c.send(ctx, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	var result ListModelsResponse
	if err := json.Unmarshal(wire.body, &result); err != nil {
		return nil, &ResponseError{RequestID: wire.requestID(), Err: err}
	}
	if result.Models == nil {
		return nil, &ResponseError{RequestID: wire.requestID(), Err: fmt.Errorf("missing models array")}
	}
	result.RequestID = wire.requestID()
	return &result, nil
}

func (c *Client) ready(ctx context.Context) error {
	if c == nil || c.config.httpClient == nil {
		return fmt.Errorf("typesafe: construct the client with NewClient")
	}
	if ctx == nil {
		return fmt.Errorf("typesafe: context must not be nil")
	}
	return ctx.Err()
}
