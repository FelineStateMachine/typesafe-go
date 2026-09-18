package typesafe

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
)

// Question is a typed System One question.
//
// The concrete question types are NoulQuestion, ChoiceQuestion, and
// ScoreQuestion. Question values always include their wire type discriminator.
type Question interface {
	questionType() string
	jsonv2.Marshaler
}

// NoulCriteria describes the true and false outcomes of a Noul question.
type NoulCriteria struct {
	True  any `json:"true"`
	False any `json:"false"`
}

// NoulQuestion asks for the probability of a yes answer.
type NoulQuestion struct {
	Instructions any           `json:"instructions"`
	Criteria     *NoulCriteria `json:"criteria"`
}

func (NoulQuestion) questionType() string { return "noul" }

// ChoiceQuestion selects one label from Criteria.
type ChoiceQuestion struct {
	Instructions any            `json:"instructions"`
	Criteria     map[string]any `json:"criteria"`
}

func (ChoiceQuestion) questionType() string { return "choice" }

// ScoreQuestion estimates a score using ordered rubric levels in Criteria.
type ScoreQuestion struct {
	Instructions any   `json:"instructions"`
	Criteria     []any `json:"criteria"`
}

func (ScoreQuestion) questionType() string { return "score" }

func marshalQuestion(kind string, value any) ([]byte, error) {
	payload, err := jsonv2.Marshal(value)
	if err != nil {
		return nil, err
	}
	var object map[string]jsontext.Value
	if err := jsonv2.Unmarshal(payload, &object); err != nil {
		return nil, fmt.Errorf("marshal %s question: %w", kind, err)
	}
	object["type"] = jsontext.Value(fmt.Sprintf("%q", kind))
	return jsonv2.Marshal(object)
}

// MarshalJSON encodes a NoulQuestion with its discriminator.
func (q NoulQuestion) MarshalJSON() ([]byte, error) {
	return marshalQuestion(q.questionType(), struct {
		Instructions any           `json:"instructions"`
		Criteria     *NoulCriteria `json:"criteria"`
	}{q.Instructions, q.Criteria})
}

// MarshalJSON encodes a ChoiceQuestion with its discriminator.
func (q ChoiceQuestion) MarshalJSON() ([]byte, error) {
	return marshalQuestion(q.questionType(), struct {
		Instructions any            `json:"instructions"`
		Criteria     map[string]any `json:"criteria"`
	}{q.Instructions, q.Criteria})
}

// MarshalJSON encodes a ScoreQuestion with its discriminator.
func (q ScoreQuestion) MarshalJSON() ([]byte, error) {
	return marshalQuestion(q.questionType(), struct {
		Instructions any   `json:"instructions"`
		Criteria     []any `json:"criteria"`
	}{q.Instructions, q.Criteria})
}

// UnmarshalQuestion decodes a discriminated question.
func UnmarshalQuestion(data []byte) (Question, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := jsonv2.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode question discriminator: %w", err)
	}
	switch envelope.Type {
	case "noul":
		var value NoulQuestion
		if err := jsonv2.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return value, nil
	case "choice":
		var value ChoiceQuestion
		if err := jsonv2.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return value, nil
	case "score":
		var value ScoreQuestion
		if err := jsonv2.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unknown question type %q", envelope.Type)
	}
}

func validateQuestion(name string, question Question) error {
	if question == nil {
		return fmt.Errorf("question %q is nil", name)
	}
	switch value := question.(type) {
	case *NoulQuestion:
		if value == nil {
			return fmt.Errorf("question %q is typed nil", name)
		}
		return validateQuestion(name, *value)
	case *ChoiceQuestion:
		if value == nil {
			return fmt.Errorf("question %q is typed nil", name)
		}
		return validateQuestion(name, *value)
	case *ScoreQuestion:
		if value == nil {
			return fmt.Errorf("question %q is typed nil", name)
		}
		return validateQuestion(name, *value)
	}
	return validateQuestionValue(name, question)
}

func validateQuestionValue(name string, question Question) error {
	switch value := question.(type) {
	case NoulQuestion:
		if err := validateTopLevelValue(value.Instructions); err != nil {
			return fmt.Errorf("question %q instructions: %w", name, err)
		}
		if value.Criteria != nil {
			if err := validateTopLevelValue(value.Criteria.True); err != nil {
				return fmt.Errorf("question %q criteria.true: %w", name, err)
			}
			if err := validateTopLevelValue(value.Criteria.False); err != nil {
				return fmt.Errorf("question %q criteria.false: %w", name, err)
			}
		}
	case ChoiceQuestion:
		if len(value.Criteria) < 2 {
			return fmt.Errorf("question %q choice criteria must contain at least two labels", name)
		}
		if err := validateTopLevelValue(value.Instructions); err != nil {
			return fmt.Errorf("question %q instructions: %w", name, err)
		}
		for label, criterion := range value.Criteria {
			if err := validateTopLevelValue(criterion); err != nil {
				return fmt.Errorf("question %q criteria[%q]: %w", name, label, err)
			}
		}
	case ScoreQuestion:
		if len(value.Criteria) < 2 {
			return fmt.Errorf("question %q score criteria must contain at least two levels", name)
		}
		if err := validateTopLevelValue(value.Instructions); err != nil {
			return fmt.Errorf("question %q instructions: %w", name, err)
		}
		for index, criterion := range value.Criteria {
			if err := validateTopLevelValue(criterion); err != nil {
				return fmt.Errorf("question %q criteria[%d]: %w", name, index, err)
			}
		}
	default:
		return fmt.Errorf("question %q has unsupported type %T", name, question)
	}
	return nil
}

func rawObject(data []byte) (map[string]jsontext.Value, error) {
	var object map[string]jsontext.Value
	if err := jsonv2.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("expected JSON object")
	}
	return object, nil
}

func hasRaw(object map[string]jsontext.Value, key string) bool {
	value, ok := object[key]
	return ok && !bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}
