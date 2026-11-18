package security

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// Accepted risks live in the plugin KV under
// "cluster/<clusterID>/accepted/<finding key>", shared by every user of
// the cluster. They outlive scans and restarts; one whose object is gone
// stays listed (GET /accepted) until someone removes it.

const (
	maxReason = 500
	maxKey    = 400 // the KV key, with its prefix, stays under 512 bytes
	maxBody   = 16 << 10
)

// acceptRecord is the stored form of an accepted risk.
type acceptRecord struct {
	CheckID  string   `json:"checkId"`
	Resource Resource `json:"resource"`
	Acceptance
}

// valid reports whether the acceptance still holds at now.
func (a Acceptance) valid(now time.Time) bool { return a.ExpiresAt == nil || now.Before(*a.ExpiresAt) }

func acceptPrefix(clusterID int64) string {
	return "cluster/" + strconv.FormatInt(clusterID, 10) + "/accepted/"
}

// parseKey checks a finding key ("<checkId>:<Kind>:<namespace>:<name>")
// and returns the check and object it names.
func parseKey(key string) (string, Resource, error) {
	parts := strings.SplitN(key, ":", 4)
	switch {
	case len(key) > maxKey:
		return "", Resource{}, reason.New(reasonKeyLength, reason.Params{"max": maxKey})
	case len(parts) != 4 || parts[1] == "" || parts[3] == "":
		return "", Resource{}, reason.New(reasonKeyFormat, nil)
	case checksByID[parts[0]] == nil:
		return "", Resource{}, reason.New(reasonKeyCheck, reason.Params{"check": parts[0]})
	case kindAPIVersions[parts[1]] == "":
		return "", Resource{}, reason.New(reasonKeyKind, reason.Params{"kind": parts[1]})
	}
	return parts[0], Resource{APIVersion: kindAPIVersions[parts[1]], Kind: parts[1], Namespace: parts[2], Name: parts[3]}, nil
}

// kindAPIVersions are the kinds findings are about.
var kindAPIVersions = map[string]string{
	"Pod": "v1", "Namespace": "v1", "ServiceAccount": "v1",
	"Deployment": "apps/v1", "StatefulSet": "apps/v1", "DaemonSet": "apps/v1", "ReplicaSet": "apps/v1",
	"Job": "batch/v1", "CronJob": "batch/v1",
	"Role": rbacAPIVersion, "ClusterRole": rbacAPIVersion, "RoleBinding": rbacAPIVersion, "ClusterRoleBinding": rbacAPIVersion,
}

// loadAccepted reads the cluster's accepted risks, by finding key. Records
// that cannot be decoded are skipped (and logged) rather than failing
// every report.
func loadAccepted(kv plugin.KV, clusterID int64) (map[string]acceptRecord, error) {
	prefix := acceptPrefix(clusterID)
	entries, err := kv.List(prefix)
	if err != nil {
		return nil, err
	}
	out := make(map[string]acceptRecord, len(entries))
	for k, v := range entries {
		var rec acceptRecord
		if err := json.Unmarshal([]byte(v), &rec); err != nil {
			log.Printf("[security] skipping accepted risk %s: %v", k, err)
			continue
		}
		out[strings.TrimPrefix(k, prefix)] = rec
	}
	return out, nil
}

func acceptances(records map[string]acceptRecord) map[string]Acceptance {
	out := make(map[string]Acceptance, len(records))
	for k, r := range records {
		out[k] = r.Acceptance
	}
	return out
}

// acceptRequest is the body of POST /accept.
type acceptRequest struct {
	Key       string     `json:"key"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

func (p *Plugin) accept(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var req acceptRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, reason.New(reason.InvalidJSON, reason.Params{"detail": err.Error()}))
		return
	}
	checkID, res, err := parseKey(req.Key)
	if err != nil {
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, err)
		return
	}
	now := p.clock()
	req.Reason = strings.TrimSpace(req.Reason)
	switch {
	case req.Reason == "":
		err = reason.New(reason.FieldRequired, reason.Params{"field": "reason"})
	case utf8.RuneCountInString(req.Reason) > maxReason:
		err = reason.New(reason.FieldTooLong, reason.Params{"field": "reason", "max": maxReason})
	case req.ExpiresAt != nil && !req.ExpiresAt.After(now):
		err = reason.New(reasonExpiresPast, nil)
	}
	if err != nil {
		plugin.WriteError(w, http.StatusBadRequest, plugin.CodeBadRequest, err)
		return
	}
	detail := map[string]any{"checkId": checkID}
	if req.ExpiresAt != nil {
		utc := req.ExpiresAt.UTC()
		req.ExpiresAt = &utc
		detail["expiresAt"] = utc.Format(time.RFC3339)
	}
	if c.SetAudit != nil {
		d, _ := json.Marshal(detail)
		c.SetAudit("finding/"+req.Key, string(d))
	}
	rec := acceptRecord{CheckID: checkID, Resource: res, Acceptance: Acceptance{
		By: c.User.Username, Reason: req.Reason, At: now.UTC(), ExpiresAt: req.ExpiresAt,
	}}
	value, err := json.Marshal(rec)
	if err == nil {
		err = c.KV.Set(acceptPrefix(c.Runtime.Cluster.ID)+req.Key, string(value))
	}
	if err != nil {
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
		return
	}
	plugin.WriteJSON(w, rec.Acceptance)
}

func (p *Plugin) unaccept(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	key := r.PathValue("key")
	kvKey := acceptPrefix(c.Runtime.Cluster.ID) + key
	_, ok, err := c.KV.Get(kvKey)
	if err == nil && ok {
		err = c.KV.Delete(kvKey)
	}
	switch {
	case err != nil:
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
	case !ok:
		plugin.WriteError(w, http.StatusNotFound, plugin.CodeNotFound, reason.New(reasonNoAccepted, reason.Params{"key": key}))
	default:
		plugin.WriteJSON(w, map[string]bool{"ok": true})
	}
}

// AcceptedRisk is one entry of GET /accepted.
type AcceptedRisk struct {
	Key      string   `json:"key"`
	CheckID  string   `json:"checkId"`
	Resource Resource `json:"resource"`
	Acceptance
	Expired bool `json:"expired"`
}

func (p *Plugin) listAccepted(w http.ResponseWriter, _ *http.Request, c *plugin.Context) {
	records, err := loadAccepted(c.KV, c.Runtime.Cluster.ID)
	if err != nil {
		plugin.WriteError(w, http.StatusInternalServerError, plugin.CodeInternal, err)
		return
	}
	now := p.clock()
	out := make([]AcceptedRisk, 0, len(records))
	for key, rec := range records {
		out = append(out, AcceptedRisk{Key: key, CheckID: rec.CheckID, Resource: rec.Resource, Acceptance: rec.Acceptance, Expired: !rec.valid(now)})
	}
	slices.SortFunc(out, func(a, b AcceptedRisk) int {
		if c := b.At.Compare(a.At); c != 0 {
			return c
		}
		return strings.Compare(a.Key, b.Key)
	})
	plugin.WriteJSON(w, out)
}
