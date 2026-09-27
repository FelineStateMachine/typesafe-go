package typesafe

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is the hosted TypeSafe API root.
const DefaultBaseURL = "https://api.typesafe.ai"

// DefaultModel is used when neither the request nor the client specifies a model.
const DefaultModel = "jev-latest"

// Version is the SDK version sent in diagnostic request headers.
const Version = "0.2.0"

type clientConfig struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	timeout    time.Duration
	retry      RetryPolicy
	headers    http.Header
}

// Option configures a client. Use the With functions to construct options.
// Options are applied in order, after environment variables and defaults.
type Option func(*clientConfig) error

func defaultConfig() clientConfig {
	return clientConfig{
		apiKey:     strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")),
		baseURL:    environmentOr("TYPESAFE_BASE_URL", DefaultBaseURL),
		model:      environmentOr("TYPESAFE_DEFAULT_MODEL", DefaultModel),
		httpClient: &http.Client{},
		timeout:    10 * time.Second,
		retry:      DefaultRetryPolicy(),
		headers:    make(http.Header),
	}
}

func environmentOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// WithAPIKey sets the API key, overriding TYPESAFE_API_KEY.
func WithAPIKey(key string) Option {
	return func(config *clientConfig) error {
		config.apiKey = strings.TrimSpace(key)
		return nil
	}
}

// WithBaseURL sets the API root, overriding TYPESAFE_BASE_URL. A path prefix is
// supported. The URL must use HTTP or HTTPS and contain no credentials, query,
// or fragment. Use HTTPS for remote services.
func WithBaseURL(baseURL string) Option {
	return func(config *clientConfig) error {
		config.baseURL = strings.TrimSpace(baseURL)
		return nil
	}
}

// WithModel sets the default model, overriding TYPESAFE_DEFAULT_MODEL.
// A nonempty SystemOneRequest.Model takes precedence over this setting.
func WithModel(model string) Option {
	return func(config *clientConfig) error {
		config.model = strings.TrimSpace(model)
		return nil
	}
}

// WithHTTPClient sets the HTTP client. The SDK takes a shallow copy and disables
// redirects; it does not mutate or close the caller's client or transport.
// A shorter HTTP client timeout also applies to each attempt.
func WithHTTPClient(client *http.Client) Option {
	return func(config *clientConfig) error {
		if client == nil {
			return fmt.Errorf("typesafe: HTTP client must not be nil")
		}
		config.httpClient = client
		return nil
	}
}

// WithTimeout sets the positive timeout for one HTTP attempt, including reading
// the response body. The default is 10 seconds. Use a context deadline to bound
// the whole operation, including retries and backoff.
func WithTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("typesafe: timeout must be positive")
		}
		config.timeout = timeout
		return nil
	}
}

// WithMaxRetries sets retries after the initial attempt. Zero disables retries.
// It changes only MaxRetries in the current retry policy.
func WithMaxRetries(maxRetries int) Option {
	return func(config *clientConfig) error {
		if maxRetries < 0 {
			return fmt.Errorf("typesafe: max retries must not be negative")
		}
		config.retry.MaxRetries = maxRetries
		return nil
	}
}

// WithRetryPolicy replaces the complete retry policy. Start from
// DefaultRetryPolicy to change selected settings while retaining the defaults.
func WithRetryPolicy(policy RetryPolicy) Option {
	return func(config *clientConfig) error {
		if err := policy.validate(); err != nil {
			return err
		}
		config.retry = policy
		return nil
	}
}

// WithHeaders copies additional headers. Authentication, JSON content headers,
// SDK identity headers, and retry count are always controlled by the SDK.
func WithHeaders(headers http.Header) Option {
	cloned := canonicalHeaders(headers)
	return func(config *clientConfig) error {
		config.headers = cloned.Clone()
		return nil
	}
}

func canonicalHeaders(headers http.Header) http.Header {
	result := make(http.Header, len(headers))
	for key, values := range headers {
		for _, value := range values {
			result.Add(key, value)
		}
	}
	return result
}

func (config *clientConfig) validate() error {
	if config.apiKey == "" {
		return fmt.Errorf("typesafe: API key required; use WithAPIKey or TYPESAFE_API_KEY")
	}
	if strings.ContainsAny(config.apiKey, "\r\n") {
		return fmt.Errorf("typesafe: API key must not contain line breaks")
	}
	if config.model == "" {
		return fmt.Errorf("typesafe: default model must not be empty")
	}
	parsed, err := url.Parse(config.baseURL)
	if err != nil {
		return fmt.Errorf("typesafe: invalid base URL")
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return fmt.Errorf("typesafe: base URL must be an HTTP(S) URL without credentials, query, or fragment")
	}
	config.baseURL = strings.TrimRight(config.baseURL, "/")
	return config.retry.validate()
}
