package typesafe_test

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/FelineStateMachine/typesafe-go"
)

func ExampleClient_SystemOne() {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if err := json.MarshalWrite(writer, map[string]any{
			"model": "jev-latest",
			"answers": map[string]any{
				"ready": map[string]any{"type": "noul", "noul": 0.9},
				"kind":  map[string]any{"type": "choice", "choice": "stable", "confidence": 0.8, "probabilities": map[string]float64{"stable": 0.8, "beta": 0.2}},
				"score": map[string]any{"type": "score", "score": 1.6, "confidence": 0.7, "legend": map[string]any{"0": "not ready", "1": "review", "2": "ready"}, "probabilities": map[string]float64{"0": 0.1, "1": 0.2, "2": 0.7}},
			},
			"usage": map[string]int{"input_tokens": 12, "output_tokens": 3},
		}); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	client, err := typesafe.NewClient(
		typesafe.WithAPIKey("test-key"),
		typesafe.WithBaseURL(server.URL),
	)
	if err != nil {
		panic(err)
	}
	response, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State: map[string]any{"text": "ready"},
		Questions: map[string]typesafe.Question{
			"ready": typesafe.NoulQuestion{Instructions: "Is it ready?", Criteria: &typesafe.NoulCriteria{True: "yes", False: "no"}},
			"kind":  typesafe.ChoiceQuestion{Instructions: "Which kind?", Criteria: map[string]any{"stable": "stable", "beta": "beta"}},
			"score": typesafe.ScoreQuestion{Instructions: "How ready?", Criteria: []any{"not ready", "review", "ready"}},
		},
	})
	if err != nil {
		panic(err)
	}
	ready, err := response.Noul("ready")
	if err != nil {
		panic(err)
	}
	kind, err := response.Choice("kind")
	if err != nil {
		panic(err)
	}
	score, err := response.Score("score")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%.1f %s %.1f\n", ready.Noul, kind.Choice, score.Score)
	// Output:
	// 0.9 stable 1.6
}
