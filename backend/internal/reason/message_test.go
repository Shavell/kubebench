package reason

import (
	"encoding/json"
	"errors"
	"testing"
)

var testCause = Define("test.cause", "the session was closed: {{cause}}")

func TestMessageOf(t *testing.T) {
	own := MessageOf(New(testCode, Params{"cluster": "prod", "count": 2}))
	if own.Code != "test.sample" || own.Params["cluster"] != "prod" || own.Text != `cluster "prod" has 2 nodes, {{missing}} unknown` {
		t.Fatalf("MessageOf(reasoned) = %+v", own)
	}
	plain := MessageOf(errors.New("pods is forbidden"))
	data, err := json.Marshal(plain)
	if err != nil || string(data) != `{"error":"pods is forbidden"}` {
		t.Fatalf("plain message = %s (%v)", data, err)
	}
}

// A Message as a parameter renders as its English text and travels as a
// nested reason the UI can translate.
func TestMessageAsParameter(t *testing.T) {
	inner := MessageOf(New(testCode, Params{"cluster": "dev", "count": 1, "missing": 0}))
	outer := New(testCause, Params{"cause": inner})
	if want := `the session was closed: cluster "dev" has 1 nodes, 0 unknown`; outer.Error() != want {
		t.Fatalf("Error = %q, want %q", outer.Error(), want)
	}
	data, err := json.Marshal(MessageOf(outer))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Reason string `json:"reason"`
		Params struct {
			Cause Message `json:"cause"`
		} `json:"params"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Reason != "test.cause" || decoded.Params.Cause.Code != "test.sample" || decoded.Params.Cause.Params["cluster"] != "dev" {
		t.Fatalf("nested reason = %s", data)
	}
}
