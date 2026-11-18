package reason

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

var testCode = Define("test.sample", "cluster \"{{cluster}}\" has {{count}} nodes, {{missing}} unknown")

func TestTextFillsPlaceholders(t *testing.T) {
	got := testCode.Text(Params{"cluster": "prod", "count": 3})
	if want := `cluster "prod" has 3 nodes, {{missing}} unknown`; got != want {
		t.Fatalf("Text = %q, want %q", got, want)
	}
	if names := Placeholders(texts[testCode.id]); strings.Join(names, ",") != "cluster,count,missing" {
		t.Fatalf("Placeholders = %v", names)
	}
}

func TestDefineRefusesMalformedAndDuplicateCodes(t *testing.T) {
	for _, id := range []string{"", "nogroup", "Upper.case", "test.sample", "test.", "a.b-c"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Define(%q) did not panic", id)
				}
			}()
			Define(id, "text")
		}()
	}
}

func TestOfFindsTheReasonInAChain(t *testing.T) {
	cause := errors.New("connection refused")
	err := fmt.Errorf("wrapped: %w", Wrap(cause, testCode, Params{"cluster": "dev"}))
	code, params, ok := Of(err)
	if !ok || code != testCode || params["cluster"] != "dev" {
		t.Fatalf("Of = %v %v %v", code, params, ok)
	}
	if !errors.Is(err, cause) {
		t.Fatal("the cause is lost")
	}
	if _, _, ok := Of(cause); ok {
		t.Fatal("a plain error has no reason")
	}
	if _, _, ok := Of(nil); ok {
		t.Fatal("nil has no reason")
	}
}

func TestFromFallsBackToDetail(t *testing.T) {
	got := From(errors.New("pods is forbidden"))
	if got.Code != Detail || got.Params["detail"] != "pods is forbidden" || got.Error() != "pods is forbidden" {
		t.Fatalf("From = %+v (%q)", got, got.Error())
	}
	if own := From(New(testCode, Params{"cluster": "x"})); own.Code != testCode {
		t.Fatalf("From kept %v, want the own reason", own.Code)
	}
}

func TestLookup(t *testing.T) {
	if code, ok := Lookup("test.sample"); !ok || code != testCode {
		t.Fatalf("Lookup = %v %v", code, ok)
	}
	if _, ok := Lookup("test.unknown"); ok {
		t.Fatal("an unknown code was found")
	}
}
