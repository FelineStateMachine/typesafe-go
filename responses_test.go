package typesafe

import (
	jsonv2 "encoding/json/v2"
	"errors"
	"testing"
)

func TestResponseUnmarshalAndAccessors(t *testing.T) {
	input := []byte(`{"model":"jev-latest","answers":{"a":{"type":"noul","noul":0},"b":{"type":"choice","choice":"yes","confidence":0.5,"probabilities":{"yes":1}},"c":{"type":"score","score":0,"confidence":0,"legend":{"0":"none","1":"yes"},"probabilities":{"0":1,"1":0}},"future":{"type":"future","value":0}},"usage":{"input_tokens":0,"output_tokens":2}}`)
	var response SystemOneResponse
	if err := jsonv2.Unmarshal(input, &response); err != nil {
		t.Fatal(err)
	}
	if got, err := response.Noul("a"); err != nil || got.Noul != 0 {
		t.Fatalf("noul: %#v %v", got, err)
	}
	if got, err := response.Choice("b"); err != nil || got.Choice != "yes" {
		t.Fatalf("choice: %#v %v", got, err)
	}
	if got, err := response.Score("c"); err != nil || got.Score != 0 {
		t.Fatalf("score: %#v %v", got, err)
	}
	var roundtrip map[string]any
	encoded, err := jsonv2.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if err := jsonv2.Unmarshal(encoded, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if _, ok := roundtrip["answers"].(map[string]any)["future"]; !ok {
		t.Fatal("unknown answer was not preserved")
	}
}

func TestResponseRejectsMissingAndInvalidFields(t *testing.T) {
	cases := []string{
		`{"answers":{},"usage":{"input_tokens":0,"output_tokens":0}}`,
		`{"model":"x","answers":{},"usage":{"input_tokens":0}}`,
		`{"model":"x","answers":{"a":{"type":"noul"}},"usage":{"input_tokens":0,"output_tokens":0}}`,
		`{"model":"x","answers":{"a":{"type":"noul","noul":1.1}},"usage":{"input_tokens":0,"output_tokens":0}}`,
	}
	for _, input := range cases {
		var response SystemOneResponse
		if err := jsonv2.Unmarshal([]byte(input), &response); err == nil {
			t.Fatalf("expected error for %s", input)
		}
	}
}

func TestResponseRejectsDuplicateAndInvalidUTF8(t *testing.T) {
	duplicate := []byte(`{"model":"x","model":"y","answers":{},"usage":{"input_tokens":0,"output_tokens":0}}`)
	if err := jsonv2.Unmarshal(duplicate, new(SystemOneResponse)); err == nil {
		t.Fatal("duplicate field should be rejected")
	}
	invalidUTF8 := []byte{'{', '"', 'm', 'o', 'd', 'e', 'l', '"', ':', '"', 0xff, '"', ',', '"', 'a', 'n', 's', 'w', 'e', 'r', 's', '"', ':', '{', '}', ',', '"', 'u', 's', 'a', 'g', 'e', '"', ':', '{', '"', 'i', 'n', 'p', 'u', 't', '_', 't', 'o', 'k', 'e', 'n', 's', '"', ':', '0', ',', '"', 'o', 'u', 't', 'p', 'u', 't', '_', 't', 'o', 'k', 'e', 'n', 's', '"', ':', '0', '}', '}'}
	if err := jsonv2.Unmarshal(invalidUTF8, new(SystemOneResponse)); err == nil {
		t.Fatal("invalid UTF-8 should be rejected")
	}
}

func TestResponseAccessorErrors(t *testing.T) {
	response := SystemOneResponse{Answers: map[string]Answer{"a": ChoiceAnswer{Choice: "x"}}}
	if _, err := response.Noul("missing"); !errors.Is(err, ErrAnswerNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := response.Noul("a"); !errors.Is(err, ErrAnswerType) {
		t.Fatalf("type: %v", err)
	}
}

func TestGenericAnswerAccessor(t *testing.T) {
	response := SystemOneResponse{Answers: map[string]Answer{"a": NoulAnswer{Noul: 0}}}
	got, err := response.Answer[NoulAnswer]("a")
	if err != nil || got.Noul != 0 {
		t.Fatalf("got %#v, %v", got, err)
	}
	if _, err := response.Answer[ChoiceAnswer]("a"); !errors.Is(err, ErrAnswerType) {
		t.Fatalf("wrong type: %v", err)
	}
}

func TestResponseRejectsInvalidProbabilityMaps(t *testing.T) {
	for _, probabilities := range []string{`{}`, `{"yes":null}`} {
		input := `{"model":"x","answers":{"a":{"type":"choice","choice":"yes","confidence":0,"probabilities":` + probabilities + `}},"usage":{"input_tokens":0,"output_tokens":0}}`
		if err := jsonv2.Unmarshal([]byte(input), new(SystemOneResponse)); err == nil {
			t.Fatalf("expected invalid probabilities error for %s", probabilities)
		}
	}
}

func TestModelListRejectsMalformedCards(t *testing.T) {
	for _, input := range []string{`{"models":[{}]}`, `{"models":[null]}`, `{"models":[{"name":"","description":"x","release_date":"today"}]}`} {
		if err := jsonv2.Unmarshal([]byte(input), new(ListModelsResponse)); err == nil {
			t.Fatalf("expected malformed model error for %s", input)
		}
	}
}
