package helm

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"

	"kubebench/internal/reason"
)

const (
	// maxUploadBytes bounds an uploaded chart archive (§7.4).
	maxUploadBytes = 20 << 20
	// uploadTTL is how long an uploaded chart can be installed from.
	uploadTTL = 24 * time.Hour
	// uploadSweep is how often expired uploads are removed.
	uploadSweep = time.Hour
)

var uploadIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Upload is POST /charts/upload's answer: the id to install from and the
// chart the archive holds.
type Upload struct {
	UploadID  string    `json:"uploadId"`
	Chart     ChartInfo `json:"chart"`
	ExpiresAt string    `json:"expiresAt"`
}

// saveUpload validates archive as a chart and stores it under a new random
// id. archive is at most maxUploadBytes (the handler limits the body).
func (e *environment) saveUpload(archive []byte, now time.Time) (*Upload, error) {
	ch, err := loader.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return nil, badRequest(reasonNotArchive, reason.Params{"detail": err.Error()})
	}
	if err := ch.Validate(); err != nil {
		return nil, badRequest(reasonUploadInvalid, reason.Params{"detail": err.Error()})
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(raw[:])
	path, err := e.uploadPath(id)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		return nil, err
	}
	return &Upload{UploadID: id, Chart: chartInfo(ch), ExpiresAt: timestamp(now.Add(uploadTTL))}, nil
}

// loadUpload loads an uploaded chart that has not expired yet.
func (e *environment) loadUpload(id string) (*chart.Chart, error) {
	path, err := e.uploadPath(id)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && time.Since(info.ModTime()) > uploadTTL) {
		return nil, notFound(reasonUploadNotFound, reason.Params{"id": id})
	}
	if err != nil {
		return nil, err
	}
	return loader.Load(path)
}

// uploadPath is the archive file of upload id inside the uploads directory.
// id comes from the request: it must be an id saveUpload hands out (32
// lowercase hex digits), so it has no separator or dot and the joined path
// cannot leave the directory. The containment check repeats that guarantee
// at the place the path is built.
func (e *environment) uploadPath(id string) (string, error) {
	if !uploadIDPattern.MatchString(id) {
		return "", badRequest(reasonUploadID, nil)
	}
	path := filepath.Join(e.uploads, id+".tgz")
	if rel, err := filepath.Rel(e.uploads, path); err != nil || rel != filepath.Base(path) {
		return "", badRequest(reasonUploadID, nil)
	}
	return path, nil
}

// sweepUploads removes uploads older than uploadTTL every uploadSweep until
// ctx ends.
func (e *environment) sweepUploads(ctx context.Context) {
	ticker := time.NewTicker(uploadSweep)
	defer ticker.Stop()
	for {
		e.removeExpiredUploads(time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (e *environment) removeExpiredUploads(now time.Time) {
	entries, err := os.ReadDir(e.uploads)
	if err != nil {
		log.Printf("[helm] uploads: %v", err)
		return
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".tgz") {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) <= uploadTTL {
			continue
		}
		if err := os.Remove(filepath.Join(e.uploads, entry.Name())); err != nil {
			log.Printf("[helm] uploads: %v", err)
		}
	}
}

// readLimited reads r, failing when it holds more than limit bytes.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, badRequest(reasonArchiveSize, reason.Params{"size": limit >> 20})
	}
	return data, nil
}
