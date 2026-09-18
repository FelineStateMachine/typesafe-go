package typesafe

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testResponse = `{"model":"jev-test","answers":{"urgent":{"type":"noul","noul":0}},"usage":{"input_tokens":3,"output_tokens":1}}`

func testClient(t *testing.T, handler http.HandlerFunc, options ...Option) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	defaults := []Option{WithAPIKey("test-key"), WithBaseURL(server.URL), WithMaxRetries(0)}
	client, err := NewClient(append(defaults, options...)...)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func testRequest() SystemOneRequest {
	return SystemOneRequest{State: "a ticket", Questions: map[string]Question{
		"urgent": NoulQuestion{Instructions: "Is this urgent?"},
	}}
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := fmt.Fprint(w, body); err != nil {
		t.Error(err)
	}
}

func TestSystemOneWireContract(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			t.Errorf("unexpected route: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content type: %q", got)
		}
		var body map[string]jsontext.Value
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Error(err)
		}
		if string(body["model"]) != `"jev-custom"` || string(body["state"]) != `"a ticket"` {
			t.Errorf("unexpected body: %s", body)
		}
		w.Header().Set("x-request-id", "req_test")
		writeResponse(t, w, testResponse)
	}, WithModel("jev-custom"))
	response, err := client.SystemOne(context.Background(), testRequest())
	if err != nil {
		t.Fatal(err)
	}
	answer, err := response.Noul("urgent")
	if err != nil || answer.Noul != 0 || response.RequestID != "req_test" {
		t.Fatalf("response: %#v; answer: %#v; error: %v", response, answer, err)
	}
}

func TestListModels(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected route: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("request-id", "req_models")
		writeResponse(t, w, `{"models":[{"name":"jev-test","description":"Test model","release_date":"2026-09-17"}]}`)
	})
	response, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Models) != 1 || response.Models[0].Name != "jev-test" || response.RequestID != "req_models" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestModelOverrideAndBasePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy/v1/systemone" {
			t.Errorf("path: %q", r.URL.Path)
		}
		var body struct {
			Model string `json:"model"`
		}
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Error(err)
		}
		if body.Model != "pinned-model" {
			t.Errorf("model: %q", body.Model)
		}
		writeResponse(t, w, testResponse)
	}))
	defer server.Close()
	client, err := NewClient(WithAPIKey("key"), WithBaseURL(server.URL+"/proxy/"))
	if err != nil {
		t.Fatal(err)
	}
	request := testRequest()
	request.Model = "pinned-model"
	if _, err := client.SystemOne(context.Background(), request); err != nil {
		t.Fatal(err)
	}
}

func TestAPIErrorPreservesMetadata(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req_bad")
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeResponse(t, w, `{"detail":"private request content"}`)
	})
	_, err := client.SystemOne(context.Background(), testRequest())
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.StatusCode != 422 || apiErr.RequestID != "req_bad" || !strings.Contains(string(apiErr.Body), "private") {
		t.Fatalf("missing error metadata: %#v", apiErr)
	}
	if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "test-key") {
		t.Fatalf("error leaked content: %s", err)
	}
}

func TestMalformedResponses(t *testing.T) {
	for _, body := range []string{"", "not JSON", `null`, `{}`, testResponse + `{}`, `{"models":null}`} {
		t.Run(body, func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("x-request-id", "req_decode")
				writeResponse(t, w, body)
			})
			_, err := client.SystemOne(context.Background(), testRequest())
			responseErr, ok := errors.AsType[*ResponseError](err)
			if !ok || responseErr.RequestID != "req_decode" {
				t.Fatalf("expected response error and request ID, got %v", err)
			}
		})
	}
}

func TestCancellationAndTimeout(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}, WithTimeout(20*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.ListModels(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled context: %v", err)
	}
	if _, err := client.ListModels(context.Background()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout must wrap DeadlineExceeded: %v", err)
	}
}

func TestRedirectDoesNotReplayCredentials(t *testing.T) {
	var reached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Store(true)
	}))
	defer target.Close()
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	})
	_, err := client.SystemOne(context.Background(), testRequest())
	apiErr, ok := errors.AsType[*APIError](err)
	if reached.Load() || !ok || apiErr.StatusCode != 307 {
		t.Fatalf("redirect was followed or unexpected error: %v", err)
	}
}

func TestClientConcurrentUse(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeResponse(t, w, testResponse)
	})
	var group sync.WaitGroup
	for range 12 {
		group.Go(func() {
			if _, err := client.SystemOne(context.Background(), testRequest()); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
}
