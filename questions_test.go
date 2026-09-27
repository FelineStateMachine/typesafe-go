package typesafe

import (
	jsonv2 "encoding/json/v2"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestQuestionMarshal(t *testing.T) {
	tests := []struct {
		name string
		q    Question
		want string
	}{
		{"noul", NoulQuestion{Instructions: "Is it urgent?", Criteria: &NoulCriteria{True: "urgent", False: nil}}, `{"type":"noul","instructions":"Is it urgent?","criteria":{"true":"urgent","false":null}}`},
		{"choice", ChoiceQuestion{Instructions: "Pick one", Criteria: map[string]any{"a": nil, "b": "B"}}, `{"type":"choice","instructions":"Pick one","criteria":{"a":null,"b":"B"}}`},
		{"score", ScoreQuestion{Instructions: "Rate it", Criteria: []any{"low", "high"}}, `{"type":"score","instructions":"Rate it","criteria":["low","high"]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonv2.Marshal(tt.q)
			if err != nil {
				t.Fatal(err)
			}
			var gotValue, wantValue any
			if err := jsonv2.Unmarshal(got, &gotValue); err != nil {
				t.Fatal(err)
			}
			if err := jsonv2.Unmarshal([]byte(tt.want), &wantValue); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotValue, wantValue) {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestQuestionUnmarshal(t *testing.T) {
	q, err := UnmarshalQuestion([]byte(`{"type":"choice","instructions":"Pick","criteria":{"yes":null,"no":null}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := q.(ChoiceQuestion); !ok {
		t.Fatalf("got %T", q)
	}
	if _, err := UnmarshalQuestion([]byte(`{"type":"future","instructions":"x"}`)); err == nil || !strings.Contains(err.Error(), "unknown question type") {
		t.Fatalf("got %v", err)
	}
}

func TestRequestValidate(t *testing.T) {
	valid := SystemOneRequest{State: map[string]any{"x": "y"}, Questions: map[string]Question{"q": NoulQuestion{Instructions: "x"}}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []SystemOneRequest{
		{State: 1, Questions: map[string]Question{"q": NoulQuestion{Instructions: "x"}}},
		{State: nil, Questions: map[string]Question{}},
		{State: nil, Questions: map[string]Question{"q": ChoiceQuestion{Criteria: map[string]any{"only": nil}}}},
		{State: nil, Questions: map[string]Question{"q": ScoreQuestion{Criteria: []any{"only"}}}},
	}
	for i, request := range cases {
		if err := request.Validate(); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}

func TestRequestValidateServiceLimits(t *testing.T) {
	labels := func(n int) map[string]any {
		criteria := make(map[string]any, n)
		for i := range n {
			criteria[fmt.Sprintf("o%d", i)] = nil
		}
		return criteria
	}
	levels := func(n int) []any {
		criteria := make([]any, n)
		for i := range criteria {
			criteria[i] = fmt.Sprintf("l%d", i)
		}
		return criteria
	}
	var nilState *requestState
	tests := []struct {
		name    string
		state   any
		q       Question
		wantErr string
	}{
		{"max choice labels", "x", ChoiceQuestion{Instructions: "x", Criteria: labels(MaxChoiceOptions)}, ""},
		{"too many choice labels", "x", ChoiceQuestion{Instructions: "x", Criteria: labels(MaxChoiceOptions + 1)}, "at most 255 labels"},
		{"max score levels", "x", ScoreQuestion{Instructions: "x", Criteria: levels(MaxScoreLevels)}, ""},
		{"too many score levels", "x", ScoreQuestion{Instructions: "x", Criteria: levels(MaxScoreLevels + 1)}, "at most 10 levels"},
		{"null score level", "x", ScoreQuestion{Instructions: "x", Criteria: []any{"low", nil}}, "must not be null"},
		{"null choice instructions", "x", ChoiceQuestion{Criteria: labels(2)}, ""},
		{"noul criteria only", "x", NoulQuestion{Criteria: &NoulCriteria{True: "yes"}}, ""},
		{"noul without description", "x", NoulQuestion{Criteria: &NoulCriteria{}}, "requires instructions or criteria"},
		{"noul empty", "x", NoulQuestion{}, "requires instructions or criteria"},
		{"nil state", nil, NoulQuestion{Instructions: "x"}, "state is required"},
		{"nil pointer state", nilState, NoulQuestion{Instructions: "x"}, "state is required"},
		{"empty string state", "", NoulQuestion{Instructions: "x"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SystemOneRequest{State: tt.state, Questions: map[string]Question{"q": tt.q}}.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

type requestState struct {
	Name string `json:"name"`
}

type scalarState struct{}

func (scalarState) MarshalJSON() ([]byte, error) { return []byte("true"), nil }

func TestRequestValidateStructuredState(t *testing.T) {
	for _, state := range []any{requestState{Name: "ok"}, &requestState{Name: "ok"}, []string{"ok"}} {
		request := SystemOneRequest{State: state, Questions: map[string]Question{"q": NoulQuestion{Instructions: "x"}}}
		if err := request.Validate(); err != nil {
			t.Fatalf("state %T: %v", state, err)
		}
	}
	request := SystemOneRequest{State: scalarState{}, Questions: map[string]Question{"q": NoulQuestion{Instructions: "x"}}}
	if err := request.Validate(); err == nil {
		t.Fatal("custom scalar state should be rejected")
	}
}

func TestQuestionRejectsDuplicateAndInvalidUTF8(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"type":"noul","type":"choice"}`),
		[]byte{'{', '"', 't', 'y', 'p', 'e', '"', ':', '"', 0xff, '"', '}'},
	} {
		if _, err := UnmarshalQuestion(data); err == nil {
			t.Fatalf("expected malformed JSON error for %q", data)
		}
	}
}

func TestRequestValidateQuestionPointers(t *testing.T) {
	noul := &NoulQuestion{Instructions: "x"}
	choice := &ChoiceQuestion{Instructions: "x", Criteria: map[string]any{"a": nil, "b": nil}}
	score := &ScoreQuestion{Instructions: "x", Criteria: []any{"a", "b"}}
	for name, question := range map[string]Question{"noul": noul, "choice": choice, "score": score} {
		request := SystemOneRequest{State: "x", Questions: map[string]Question{name: question}}
		if err := request.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	var nilNoul *NoulQuestion
	if err := (SystemOneRequest{State: "x", Questions: map[string]Question{"q": nilNoul}}).Validate(); err == nil {
		t.Fatal("typed nil question should be rejected")
	}
}
