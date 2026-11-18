package plugin

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"kubebench/internal/reason"
)

func TestWriteErrorAddsTheReason(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", reason.New(reason.FieldRequired, reason.Params{"field": "name"}))
	status, body := errorOf(t, func(w http.ResponseWriter) { WriteError(w, http.StatusBadRequest, CodeBadRequest, err) })
	if status != http.StatusBadRequest || body.Code != CodeBadRequest || body.Reason != "field.required" ||
		body.Params["field"] != "name" || body.Error != "wrapped: name is required" {
		t.Fatalf("%d %+v", status, body)
	}
	_, plain := errorOf(t, func(w http.ResponseWriter) { WriteError(w, http.StatusBadRequest, CodeBadRequest, errors.New("x")) })
	if plain.Reason != "" || plain.Params != nil {
		t.Fatalf("a plain error got a reason: %+v", plain)
	}
}

func TestUnavailableStatus(t *testing.T) {
	st := Unavailable(reason.FieldRequired, reason.Params{"field": "x"})
	if st.Available || st.Reason != "x is required" || st.ReasonCode != "field.required" || st.ReasonParams["field"] != "x" {
		t.Fatalf("Unavailable = %+v", st)
	}
	if st := UnavailableFrom(errors.New("boom")); st.Reason != "boom" || st.ReasonCode != "" {
		t.Fatalf("UnavailableFrom = %+v", st)
	}
}
