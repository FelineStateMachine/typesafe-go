# typesafe-go

An unofficial, community-maintained Go SDK for the hosted [TypeSafe](https://typesafe.ai/) API.
It is an independent project by FelineStateMachine, with no official affiliation
with or endorsement from TypeSafe. The SDK sends state and questions to the
hosted service; it does not run inference locally.

The module is pure Go and uses only the standard library.

```sh
go get github.com/FelineStateMachine/typesafe-go@v0.1.0
```

Documentation: [pkg.go.dev/github.com/FelineStateMachine/typesafe-go](https://pkg.go.dev/github.com/FelineStateMachine/typesafe-go)

## Quickstart

Set `TYPESAFE_API_KEY` and create a client. `NewClient` also accepts
`WithAPIKey` when applications prefer explicit configuration.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/FelineStateMachine/typesafe-go"
)

func main() {
	client, err := typesafe.NewClient()
	if err != nil {
		log.Fatal(err)
	}

	result, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State: map[string]any{"text": "The package has tests and documentation."},
		Questions: map[string]typesafe.Question{
			"is_ready": typesafe.NoulQuestion{
				Instructions: "Is this ready to publish?",
				Criteria: &typesafe.NoulCriteria{
					True:  "The package is ready for users.",
					False: "The package still needs substantial work.",
				},
			},
			"release_channel": typesafe.ChoiceQuestion{
				Instructions: "Which release channel fits?",
				Criteria: map[string]any{
					"stable": "Ready for a stable release.",
					"beta":   "Useful, but still needs real-world feedback.",
				},
			},
			"readiness": typesafe.ScoreQuestion{
				Instructions: "Score the release readiness.",
				Criteria: []any{
					"Not ready",
					"Needs review",
					"Ready",
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	isReady, err := result.Noul("is_ready")
	if err != nil {
		log.Fatal(err)
	}
	channel, err := result.Choice("release_channel")
	if err != nil {
		log.Fatal(err)
	}
	readiness, err := result.Score("readiness")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ready=%v channel=%s score=%.1f\n", isReady.Noul >= 0.5, channel.Choice, readiness.Score)
}
```

The accessors check the answer's wire type and return an error when a question
has a different type, is missing, or contains malformed known fields. Unknown
answer types are retained as `UnknownAnswer` so a newer service response is not
silently discarded.

`NoulAnswer.Noul` is the service's yes probability. `ChoiceAnswer.Confidence`
and `ScoreAnswer.Confidence` describe confidence in the selected result; they
are not probabilities for the answer's truth. Treat all model output as
uncertain application data and choose thresholds appropriate to your use case.

## Models

```go
models, err := client.ListModels(context.Background())
if err != nil {
	// Handle *typesafe.APIError or *typesafe.ResponseError as appropriate.
	log.Fatal(err)
}
for _, model := range models.Models {
	fmt.Println(model.Name, model.Description)
}
```

Requests use `jev-latest` by default. Select another model with
`typesafe.WithModel("model-name")`, or set `TYPESAFE_DEFAULT_MODEL`. A request's
`SystemOneRequest.Model` takes precedence for that request.

## Configuration

`NewClient` reads these environment variables when the corresponding option
is not supplied:

| Setting | Option | Environment | Default |
| --- | --- | --- | --- |
| API key | `WithAPIKey` | `TYPESAFE_API_KEY` | required |
| Base URL | `WithBaseURL` | `TYPESAFE_BASE_URL` | `https://api.typesafe.ai` |
| Model | `WithModel` | `TYPESAFE_DEFAULT_MODEL` | `jev-latest` |
| HTTP client | `WithHTTPClient` | — | fresh `http.Client` |
| Attempt timeout | `WithTimeout` | — | 10 seconds |
| Retries | `WithMaxRetries` or `WithRetryPolicy` | — | 2 retries |

Whitespace-only environment values are ignored. Explicit options take
precedence over environment values. `WithHeaders` adds headers to requests;
the SDK sets authentication and content headers itself.

## Errors, context, and retries

Every request accepts a `context.Context`. Cancellation and an expired caller
deadline stop the request and are returned through the wrapped error chain.
`WithTimeout` limits each attempt; use the context for an overall deadline.

The default policy retries connection errors, timeouts, HTTP 408, HTTP 429,
and HTTP 5xx responses. It uses exponential backoff with jitter and honors
`retry-after-ms` and `Retry-After` up to the policy's maximum. Set a custom
`RetryPolicy` when the defaults do not fit, or `WithMaxRetries(0)` to disable
retries. Caller cancellation is never retried. Redirects are rejected to avoid
replaying API keys or state to another endpoint. When `WithHTTPClient` is used,
the client configuration is copied before the SDK installs its redirect policy;
the caller's `*http.Client` is never mutated.

HTTP failures return `*APIError`, which exposes status, request ID, headers,
method, URL, and response bytes without placing the body in its `Error()`
string. Response decoding validates required and known answer fields and
returns `*ResponseError` when it fails; the error retains the request ID when
one was supplied. The typed accessors then check only whether an answer exists
and has the requested type. Use `errors.Is` for context errors and
`errors.AsType` for typed errors:

```go
apiErr, ok := errors.AsType[*typesafe.APIError](err)
if ok {
	fmt.Println(apiErr.StatusCode, apiErr.RequestID)
}
```

Responses larger than 16 MiB are rejected before decoding. This bounds memory
use when an endpoint returns an unexpectedly large payload.

## Compatibility and scope

This release targets Go 1.27 and later, supports Go modules, and has no
third-party dependencies. The public surface covers the documented
`SystemOne` and `models` operations, typed Noul, Choice, and Score questions,
and forward-compatible handling of unknown answer types. The API contract was
checked against the TypeSafe API documentation; the documentation is a
reference for wire compatibility and does not imply that this repository is an
official TypeSafe SDK.

Go 1.27's `encoding/json/v2` is used for strict response handling: duplicate
object members and invalid UTF-8 are rejected, while unknown answer payloads
are preserved as `jsontext.Value`. In addition to the named convenience
methods, a response can be read with a generic accessor:

```go
answer, err := result.Answer[typesafe.ChoiceAnswer]("release_channel")
```

## Development

The default test suite is offline. Run the same checks used by CI with:

```sh
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
```

The live API check makes billable requests and is explicitly gated. Run it
only with a configured API key:

```sh
TYPESAFE_LIVE_TEST=1 go test -run TestLiveAPI -v
```

See [CONTRIBUTING.md](CONTRIBUTING.md), [CHANGELOG.md](CHANGELOG.md), and the
[TypeSafe API documentation](https://docs.typesafe.ai/api) for more context.

## License

MIT. See [LICENSE](LICENSE).
