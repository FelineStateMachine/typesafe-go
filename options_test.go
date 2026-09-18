package typesafe

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func clearEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"TYPESAFE_API_KEY", "TYPESAFE_BASE_URL", "TYPESAFE_DEFAULT_MODEL"} {
		t.Setenv(key, "")
	}
}

func TestClientOptionsValidate(t *testing.T) {
	clearEnvironment(t)
	if _, err := NewClient(); err == nil {
		t.Fatal("missing API key must fail")
	}
	tests := map[string]Option{
		"empty key": WithAPIKey(" "), "negative timeout": WithTimeout(-time.Second),
		"zero timeout": WithTimeout(0), "negative retries": WithMaxRetries(-1),
		"nil transport": WithHTTPClient(nil), "empty model": WithModel(" "),
		"invalid URL": WithBaseURL(":bad"), "URL credentials": WithBaseURL("https://a:b@example.com"),
		"URL query": WithBaseURL("https://example.com?x=y"), "URL fragment": WithBaseURL("https://example.com/#x"),
		"URL scheme": WithBaseURL("file:///tmp/x"), "nil option": nil,
		"bad policy": WithRetryPolicy(RetryPolicy{Jitter: 2}),
	}
	for name, option := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewClient(WithAPIKey("key"), option); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestEnvironmentAndExplicitPrecedence(t *testing.T) {
	clearEnvironment(t)
	t.Setenv("TYPESAFE_API_KEY", " env-key ")
	t.Setenv("TYPESAFE_DEFAULT_MODEL", " env-model ")
	t.Setenv("TYPESAFE_BASE_URL", "https://env.example/")
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if client.config.apiKey != "env-key" || client.config.model != "env-model" || client.config.baseURL != "https://env.example" {
		t.Fatal("environment configuration not resolved")
	}
	client, err = NewClient(WithAPIKey("explicit"), WithModel("explicit"), WithBaseURL("https://explicit.example"))
	if err != nil || client.config.apiKey != "explicit" || client.config.model != "explicit" || client.config.baseURL != "https://explicit.example" {
		t.Fatal("explicit configuration did not take precedence")
	}
}

func TestHeadersAreCopiedAndProtected(t *testing.T) {
	headers := http.Header{"X-Custom": {"original"}, "authorization": {"injected"}, "X-Typesafe-Retry-Count": {"999"}}
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom") != "original" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("headers were mutated or authentication overridden")
		}
		if r.Header.Get("X-TypeSafe-Retry-Count") != "" {
			t.Error("retry count overridden")
		}
		writeResponse(t, w, `{"models":[]}`)
	}, WithHeaders(headers))
	headers.Set("X-Custom", "mutated")
	if _, err := client.ListModels(context.Background()); err != nil {
		t.Fatal(err)
	}
}
