package typesafe_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FelineStateMachine/typesafe-go"
)

// TestLiveAPI makes billable requests only when explicitly enabled.
func TestLiveAPI(t *testing.T) {
	if os.Getenv("TYPESAFE_LIVE_TEST") != "1" {
		t.Skip("set TYPESAFE_LIVE_TEST=1 and TYPESAFE_API_KEY to run live API checks")
	}
	client, err := typesafe.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	models, err := client.ListModels(ctx)
	if err != nil || len(models.Models) == 0 {
		t.Fatalf("list models: response=%v err=%v", models, err)
	}
	response, err := client.SystemOne(ctx, liveRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := response.Answer[typesafe.NoulAnswer]("greeting"); err != nil {
		t.Fatal(err)
	}
	if _, err := response.Choice("tone"); err != nil {
		t.Fatal(err)
	}
	if _, err := response.Score("urgency"); err != nil {
		t.Fatal(err)
	}
}

func liveRequest() typesafe.SystemOneRequest {
	return typesafe.SystemOneRequest{
		State: "Hello! Could you send me a receipt whenever you have a moment?",
		Questions: map[string]typesafe.Question{
			"greeting": typesafe.NoulQuestion{Instructions: "Does the message contain a greeting?"},
			"tone": typesafe.ChoiceQuestion{
				Instructions: "What is the tone?", Criteria: map[string]any{"polite": nil, "hostile": nil},
			},
			"urgency": typesafe.ScoreQuestion{
				Instructions: "How urgent is the request?", Criteria: []any{"not urgent", "urgent"},
			},
		},
	}
}
