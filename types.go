package typesafe

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
)

// SystemOneRequest contains state and named questions to evaluate.
type SystemOneRequest struct {
	State     any                 `json:"state"`
	Questions map[string]Question `json:"questions"`
	Model     string              `json:"model,omitempty"`
}

// Validate checks the locally enforceable System One request constraints.
func (r SystemOneRequest) Validate() error {
	if kind, err := topLevelKind(r.State); err != nil {
		return fmt.Errorf("state: %w", err)
	} else if kind == 'n' {
		return fmt.Errorf("state is required and must not be null")
	}
	if len(r.Questions) == 0 {
		return fmt.Errorf("questions must contain at least one question")
	}
	for name, question := range r.Questions {
		if name == "" {
			return fmt.Errorf("question name must not be empty")
		}
		if err := validateQuestion(name, question); err != nil {
			return err
		}
	}
	return nil
}

func validateTopLevelValue(value any) error {
	_, err := topLevelKind(value)
	return err
}

// topLevelKind validates that value encodes as a JSON string, object, array,
// or null, and returns its kind so callers can apply stricter rules.
func topLevelKind(value any) (jsontext.Kind, error) {
	encoded, err := jsonv2.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("must be valid JSON: %w", err)
	}
	kind := jsontext.Value(encoded).Kind()
	if kind != 'n' && kind != '"' && kind != '{' && kind != '[' {
		return 0, fmt.Errorf("must be a JSON string, object, array, or null")
	}
	return kind, nil
}
