package kube

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

// fakeExecutor writes stdout and stderr, then returns err.
type fakeExecutor struct {
	stdout, stderr string
	err            error
}

func (f fakeExecutor) Stream(o remotecommand.StreamOptions) error {
	return f.StreamWithContext(context.Background(), o)
}

func (f fakeExecutor) StreamWithContext(_ context.Context, o remotecommand.StreamOptions) error {
	_, _ = io.WriteString(o.Stdout, f.stdout)
	_, _ = io.WriteString(o.Stderr, f.stderr)
	return f.err
}

func TestRunOneShot(t *testing.T) {
	exit := utilexec.CodeExitError{Err: errors.New("command terminated with exit code 2"), Code: 2}
	for name, tc := range map[string]struct {
		ex      fakeExecutor
		want    string
		wantErr bool
	}{
		"success":           {fakeExecutor{stdout: "a\nb\n"}, "a|b", false},
		"stderr appended":   {fakeExecutor{stdout: "out\n", stderr: "warn\n"}, "out|warn", false},
		"non-zero exit":     {fakeExecutor{stderr: "ls: nope\n", err: exit}, "ls: nope", false},
		"silent exit":       {fakeExecutor{err: exit}, "command terminated with exit code 2", false},
		"transport failure": {fakeExecutor{err: errors.New("unable to upgrade connection: pod not found")}, "", true},
		"partial then lost": {fakeExecutor{stdout: "half", err: context.DeadlineExceeded}, "", true},
	} {
		lines, err := runOneShot(context.Background(), tc.ex)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err = %v", name, err)
			continue
		}
		if !tc.wantErr && strings.Join(lines, "|") != tc.want {
			t.Errorf("%s: lines = %q, want %q", name, lines, tc.want)
		}
	}
}
