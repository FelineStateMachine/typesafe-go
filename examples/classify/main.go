package main

import (
	"context"
	"fmt"
	"log"

	"github.com/FelineStateMachine/typesafe-go"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	client, err := typesafe.NewClient()
	if err != nil {
		return err
	}
	response, err := client.SystemOne(context.Background(), typesafe.SystemOneRequest{
		State: map[string]any{"text": "A customer asks for a refund."},
		Questions: map[string]typesafe.Question{
			"needs_review": typesafe.NoulQuestion{
				Instructions: "Does this need human review?",
				Criteria: &typesafe.NoulCriteria{
					True:  "The request has risk or ambiguity.",
					False: "The request is routine and safe to automate.",
				},
			},
			"intent": typesafe.ChoiceQuestion{
				Instructions: "Classify the customer's intent.",
				Criteria: map[string]any{
					"refund":   "The customer wants money returned.",
					"question": "The customer is asking for information.",
				},
			},
			"priority": typesafe.ScoreQuestion{
				Instructions: "Score urgency from low to high.",
				Criteria:     []any{"low", "medium", "high"},
			},
		},
	})
	if err != nil {
		return err
	}
	needsReview, err := response.Noul("needs_review")
	if err != nil {
		return err
	}
	intent, err := response.Choice("intent")
	if err != nil {
		return err
	}
	priority, err := response.Score("priority")
	if err != nil {
		return err
	}
	fmt.Printf("review probability: %.3f\nintent: %s\npriority: %.1f\n", needsReview.Noul, intent.Choice, priority.Score)
	return nil
}
