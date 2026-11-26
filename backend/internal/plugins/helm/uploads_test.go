package helm

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kubebench/internal/reason"
)

// Upload ids come from the request; no id may name a file outside the
// uploads directory.
func TestUploadPathStaysInUploads(t *testing.T) {
	root := t.TempDir()
	env, err := newEnvironment(root)
	if err != nil {
		t.Fatal(err)
	}
	// A chart archive outside the uploads directory, where a traversal
	// would find it.
	outside := filepath.Join(root, "secret.tgz")
	if err := os.WriteFile(outside, chartArchive(t, testChart("secret", "1.0.0")), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{
		"../secret",
		"..",
		"../../etc/passwd",
		"/etc/passwd",
		outside,
		"..%2fsecret",
		`..\secret`,
		"0123456789abcdef0123456789abcdef/../../secret",
		"0123456789ABCDEF0123456789ABCDEF", // upper case is not an id
		"0123456789abcdef0123456789abcde",  // too short
		"0123456789abcdef0123456789abcdef0",
		"0123456789abcdef0123456789abcdef\n",
		"",
	} {
		if path, err := env.uploadPath(id); err == nil {
			t.Errorf("uploadPath(%q) = %q, want an error", id, path)
		} else if rc, _, ok := reason.Of(err); !ok || rc != reasonUploadID {
			t.Errorf("uploadPath(%q) error = %v, want %s", id, err, reasonUploadID)
		}
		if _, err := env.loadUpload(id); err == nil {
			t.Errorf("loadUpload(%q) loaded a chart", id)
		}
	}

	id := strings.Repeat("ab", 16)
	path, err := env.uploadPath(id)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "uploads", id+".tgz"); path != want {
		t.Errorf("uploadPath(%q) = %q, want %q", id, path, want)
	}
}

func TestUploadRoundTrip(t *testing.T) {
	env, err := newEnvironment(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	up, err := env.saveUpload(chartArchive(t, testChart("demo", "1.2.3")), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ch, err := env.loadUpload(up.UploadID)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Metadata.Name != "demo" {
		t.Errorf("loaded chart %q, want demo", ch.Metadata.Name)
	}
	if _, err := env.loadUpload(strings.Repeat("0", 32)); err == nil {
		t.Error("an unknown id loaded a chart")
	} else if rc, _, _ := reason.Of(err); rc != reasonUploadNotFound {
		t.Errorf("unknown id: %v, want %s", err, reasonUploadNotFound)
	} else if errors.Is(err, os.ErrNotExist) {
		t.Error("the not-found error leaks the file error")
	}
}
