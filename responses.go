package typesafe

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"math"
)

// ErrAnswerNotFound indicates that a named answer is absent.
var ErrAnswerNotFound = errors.New("answer not found")

// ErrAnswerType indicates that an answer exists but has another type.
var ErrAnswerType = errors.New("answer has unexpected type")

// Answer is a discriminated answer returned by System One.
type Answer interface {
	answerType() string
	jsonv2.Marshaler
}

// NoulAnswer contains the probability of a yes answer.
type NoulAnswer struct {
	Noul float64 `json:"noul"`
}

func (NoulAnswer) answerType() string { return "noul" }

// ChoiceAnswer contains the selected label, confidence, and label probabilities.
type ChoiceAnswer struct {
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

func (ChoiceAnswer) answerType() string { return "choice" }

// ScoreAnswer contains an expected score, rubric legend, confidence, and probabilities.
type ScoreAnswer struct {
	Score         float64            `json:"score"`
	Confidence    float64            `json:"confidence"`
	Legend        map[string]any     `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
}

func (ScoreAnswer) answerType() string { return "score" }

// UnknownAnswer preserves an answer type this SDK does not know yet.
type UnknownAnswer struct {
	Type string
	Raw  jsontext.Value
}

func (a UnknownAnswer) answerType() string           { return a.Type }
func (a UnknownAnswer) MarshalJSON() ([]byte, error) { return append([]byte(nil), a.Raw...), nil }

func marshalAnswer(kind string, value any) ([]byte, error) {
	payload, err := jsonv2.Marshal(value)
	if err != nil {
		return nil, err
	}
	object, err := rawObject(payload)
	if err != nil {
		return nil, err
	}
	object["type"] = jsontext.Value(fmt.Sprintf("%q", kind))
	return jsonv2.Marshal(object)
}

// MarshalJSON encodes a NoulAnswer with its discriminator.
func (a NoulAnswer) MarshalJSON() ([]byte, error) {
	return marshalAnswer(a.answerType(), struct {
		Noul float64 `json:"noul"`
	}{a.Noul})
}

// MarshalJSON encodes a ChoiceAnswer with its discriminator.
func (a ChoiceAnswer) MarshalJSON() ([]byte, error) {
	return marshalAnswer(a.answerType(), struct {
		Choice        string             `json:"choice"`
		Confidence    float64            `json:"confidence"`
		Probabilities map[string]float64 `json:"probabilities"`
	}{a.Choice, a.Confidence, a.Probabilities})
}

// MarshalJSON encodes a ScoreAnswer with its discriminator.
func (a ScoreAnswer) MarshalJSON() ([]byte, error) {
	return marshalAnswer(a.answerType(), struct {
		Score         float64            `json:"score"`
		Confidence    float64            `json:"confidence"`
		Legend        map[string]any     `json:"legend"`
		Probabilities map[string]float64 `json:"probabilities"`
	}{a.Score, a.Confidence, a.Legend, a.Probabilities})
}

// Usage reports token counts for a System One request.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// SystemOneResponse contains all named answers and request usage metadata.
type SystemOneResponse struct {
	Model     string            `json:"model"`
	Answers   map[string]Answer `json:"answers"`
	Usage     Usage             `json:"usage"`
	RequestID string            `json:"-"`
}

// UnmarshalAnswer decodes a discriminated answer and preserves unknown types.
func UnmarshalAnswer(data []byte) (Answer, error) {
	object, err := rawObject(data)
	if err != nil {
		return nil, err
	}
	tag, ok := object["type"]
	if !ok {
		return nil, fmt.Errorf("answer type is required")
	}
	var kind string
	if err := jsonv2.Unmarshal(tag, &kind); err != nil || kind == "" {
		return nil, fmt.Errorf("answer type must be a non-empty string")
	}
	switch kind {
	case "noul":
		var value NoulAnswer
		if err := decodeKnownAnswer(data, object, &value, kind); err != nil {
			return nil, err
		}
		return value, nil
	case "choice":
		var value ChoiceAnswer
		if err := decodeKnownAnswer(data, object, &value, kind); err != nil {
			return nil, err
		}
		return value, nil
	case "score":
		var value ScoreAnswer
		if err := decodeKnownAnswer(data, object, &value, kind); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return UnknownAnswer{Type: kind, Raw: append(jsontext.Value(nil), data...)}, nil
	}
}

func decodeKnownAnswer(data []byte, object map[string]jsontext.Value, target any, kind string) error {
	if err := jsonv2.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s answer: %w", kind, err)
	}
	switch value := target.(type) {
	case *NoulAnswer:
		if !hasRaw(object, "noul") || !finiteProbability(value.Noul) {
			return fmt.Errorf("invalid noul answer")
		}
	case *ChoiceAnswer:
		if !validChoiceAnswer(*value, object) {
			return fmt.Errorf("invalid choice answer")
		}
	case *ScoreAnswer:
		if !validScoreAnswer(*value, object) {
			return fmt.Errorf("invalid score answer")
		}
	}
	return nil
}

func validChoiceAnswer(value ChoiceAnswer, object map[string]jsontext.Value) bool {
	return hasRaw(object, "choice") && hasRaw(object, "confidence") &&
		finiteProbability(value.Confidence) && value.Choice != "" &&
		mapHasKey(value.Probabilities, value.Choice) &&
		validProbabilities(value.Probabilities, object, "probabilities")
}

func validScoreAnswer(value ScoreAnswer, object map[string]jsontext.Value) bool {
	return hasRaw(object, "score") && hasRaw(object, "confidence") &&
		hasRaw(object, "legend") && finiteProbability(value.Confidence) &&
		sameKeys(value.Legend, value.Probabilities) &&
		validProbabilities(value.Probabilities, object, "probabilities")
}

func finiteProbability(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func validProbabilities(values map[string]float64, object map[string]jsontext.Value, key string) bool {
	raw, ok := object[key]
	if !ok || !hasRaw(object, key) || len(values) == 0 {
		return false
	}
	var rawValues map[string]jsontext.Value
	if err := jsonv2.Unmarshal(raw, &rawValues); err != nil || len(rawValues) != len(values) {
		return false
	}
	for _, rawValue := range rawValues {
		if bytes.Equal(bytes.TrimSpace(rawValue), []byte("null")) {
			return false
		}
	}
	for _, value := range values {
		if !finiteProbability(value) {
			return false
		}
	}
	return true
}

func mapHasKey(values map[string]float64, key string) bool { _, ok := values[key]; return ok }

func sameKeys(legend map[string]any, probabilities map[string]float64) bool {
	if len(legend) != len(probabilities) {
		return false
	}
	for key := range legend {
		if _, ok := probabilities[key]; !ok {
			return false
		}
	}
	return true
}

// UnmarshalJSON decodes and validates a SystemOneResponse.
func (r *SystemOneResponse) UnmarshalJSON(data []byte) error {
	object, err := rawObject(data)
	if err != nil {
		return err
	}
	if !hasRaw(object, "model") || !hasRaw(object, "answers") || !hasRaw(object, "usage") {
		return fmt.Errorf("response requires model, answers, and usage")
	}
	var usageObject map[string]jsontext.Value
	if err := jsonv2.Unmarshal(object["usage"], &usageObject); err != nil || !hasRaw(usageObject, "input_tokens") || !hasRaw(usageObject, "output_tokens") {
		return fmt.Errorf("usage requires input_tokens and output_tokens")
	}
	var envelope struct {
		Model   string                    `json:"model"`
		Answers map[string]jsontext.Value `json:"answers"`
		Usage   *Usage                    `json:"usage"`
	}
	if err := jsonv2.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.Model == "" || envelope.Answers == nil || envelope.Usage == nil {
		return fmt.Errorf("response has invalid required fields")
	}
	answers := make(map[string]Answer, len(envelope.Answers))
	for name, raw := range envelope.Answers {
		answer, err := UnmarshalAnswer(raw)
		if err != nil {
			return fmt.Errorf("answer %q: %w", name, err)
		}
		answers[name] = answer
	}
	*r = SystemOneResponse{Model: envelope.Model, Answers: answers, Usage: *envelope.Usage}
	return nil
}

// Noul returns the named Noul answer.
func (r SystemOneResponse) Noul(name string) (NoulAnswer, error) {
	value, err := answerAs[NoulAnswer](r.Answers, name)
	return value, err
}

// Choice returns the named Choice answer.
func (r SystemOneResponse) Choice(name string) (ChoiceAnswer, error) {
	value, err := answerAs[ChoiceAnswer](r.Answers, name)
	return value, err
}

// Score returns the named Score answer.
func (r SystemOneResponse) Score(name string) (ScoreAnswer, error) {
	value, err := answerAs[ScoreAnswer](r.Answers, name)
	return value, err
}

// Answer returns a named answer as the requested concrete answer type.
func (r SystemOneResponse) Answer[T Answer](name string) (T, error) {
	return answerAs[T](r.Answers, name)
}

func answerAs[T Answer](answers map[string]Answer, name string) (T, error) {
	var zero T
	answer, ok := answers[name]
	if !ok {
		return zero, fmt.Errorf("%w: %s", ErrAnswerNotFound, name)
	}
	value, ok := answer.(T)
	if !ok {
		return zero, fmt.Errorf("%w: %s", ErrAnswerType, name)
	}
	return value, nil
}

// ModelCard describes an available model.
type ModelCard struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReleaseDate string `json:"release_date"`
}

// ListModelsResponse contains the models available to the account.
type ListModelsResponse struct {
	Models    []ModelCard `json:"models"`
	RequestID string      `json:"-"`
}

// UnmarshalJSON decodes and validates a model list response.
func (r *ListModelsResponse) UnmarshalJSON(data []byte) error {
	object, err := rawObject(data)
	if err != nil {
		return err
	}
	if !hasRaw(object, "models") {
		return fmt.Errorf("response requires models")
	}
	var rawModels []jsontext.Value
	if err := jsonv2.Unmarshal(object["models"], &rawModels); err != nil {
		return err
	}
	if rawModels == nil {
		return fmt.Errorf("models must be an array")
	}
	models := make([]ModelCard, len(rawModels))
	for index, rawModel := range rawModels {
		modelObject, err := rawObject(rawModel)
		if err != nil {
			return fmt.Errorf("models[%d]: %w", index, err)
		}
		if !hasRaw(modelObject, "name") || !hasRaw(modelObject, "description") || !hasRaw(modelObject, "release_date") {
			return fmt.Errorf("models[%d]: missing required fields", index)
		}
		if err := jsonv2.Unmarshal(rawModel, &models[index]); err != nil {
			return err
		}
		if models[index].Name == "" {
			return fmt.Errorf("models[%d]: name must not be empty", index)
		}
	}
	*r = ListModelsResponse{Models: models}
	return nil
}

var _ jsonv2.Marshaler = NoulAnswer{}
var _ jsonv2.Marshaler = ChoiceAnswer{}
var _ jsonv2.Marshaler = ScoreAnswer{}
var _ jsonv2.Marshaler = NoulQuestion{}
var _ jsonv2.Marshaler = ChoiceQuestion{}
var _ jsonv2.Marshaler = ScoreQuestion{}
