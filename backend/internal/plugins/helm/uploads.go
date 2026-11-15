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
		return nil, badRequest("not a chart archive: %v", err)
	}
	if err := ch.Validate(); err != nil {
		return nil, badRequest("invalid chart: %v", err)
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(raw[:])
	if err := os.WriteFile(e.uploadPath(id), archive, 0o600); err != nil {
		return nil, err
	}
	return &Upload{UploadID: id, Chart: chartInfo(ch), ExpiresAt: timestamp(now.Add(uploadTTL))}, nil
}

// loadUpload loads an uploaded chart that has not expired yet.
func (e *environment) loadUpload(id string) (*chart.Chart, error) {
	path := e.uploadPath(id)
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && time.Since(info.ModTime()) > uploadTTL) {
		return nil, notFound("upload %s does not exist or has expired; upload the chart again", id)
	}
	if err != nil {
		return nil, err
	}
	return loader.Load(path)
}

func (e *environment) uploadPath(id string) string {
	return filepath.Join(e.uploads, id+".tgz")
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
		return nil, badRequest("the chart archive is larger than %d MiB", limit>>20)
	}
	return data, nil
}
