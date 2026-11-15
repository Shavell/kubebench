package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// Resource editing (docs/design-v0.5.md §7.2): server-side apply with dry
// runs, the generic delete, label and annotation patches and the Secret
// reveal.

// maxManifestBytes bounds the body of an apply request.
const maxManifestBytes = 4 << 20

// applyResult is one object of the apply response (src/data/types.ts
// ApplyResult). A failed object has error and code instead of action.
type applyResult struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Namespace  string   `json:"namespace,omitempty"`
	Name       string   `json:"name"`
	Action     string   `json:"action,omitempty"`
	Error      string   `json:"error,omitempty"`
	Code       string   `json:"code,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// handleApply applies the YAML or JSON manifests in the body. Requests
// the manifests cannot be applied as (invalid syntax, missing names,
// redacted Secret values) are refused as a whole; after that every object
// is attempted and reports its own outcome, so the response is 200 even
// when objects failed. With dryRun=true the router only required read
// access and nothing is persisted.
func (s *Server) handleApply(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	// The router went by plugin.IsDryRun; refuse spellings it did not take
	// as a dry run rather than apply what was meant as one.
	if _, err := flagParam(q, "dryRun"); err != nil {
		writeErr(w, err)
		return
	}
	force, err := flagParam(q, "force")
	if err != nil {
		writeErr(w, err)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxManifestBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeErr(w, errorf(http.StatusRequestEntityTooLarge, plugin.CodeBadRequest,
				"the manifests exceed %d MiB", maxManifestBytes>>20))
			return
		}
		writeErr(w, badRequest("reading the manifests: %v", err))
		return
	}
	objects, err := kube.ParseManifests(data)
	if err != nil {
		writeManifestErr(w, err)
		return
	}
	if len(objects) == 0 {
		writeErr(w, badRequest("the request contains no objects"))
		return
	}

	dryRun := plugin.IsDryRun(r)
	if dryRun && touchesSecrets(c.rt, objects) {
		var done func()
		w, done, err = s.secretDryRun(w, r, c, "POST /api/resource/apply")
		defer done()
		if err != nil {
			writeErr(w, err)
			return
		}
	}
	results := c.rt.Apply(r.Context(), objects, kube.ApplyOptions{
		Namespace: q.Get("ns"),
		DryRun:    dryRun,
		Force:     force,
	})
	c.setAudit(applyAuditTarget(results), applyAuditDetail(results, force, dryRun))
	out := make([]applyResult, len(results))
	for i, res := range results {
		out[i] = toApplyResult(res)
	}
	plugin.WriteJSON(w, map[string][]applyResult{"results": out})
}

// editRequest is the body of POST /api/resource/edit: the object's YAML
// (or JSON) as editing started from, and as edited.
type editRequest struct {
	Original string `json:"original"`
	Edited   string `json:"edited"`
}

// handleEdit applies an edit of one existing object as the merge patch
// from original to edited (kube.EditObject), so that removed fields are
// removed. The answer has the shape of an apply's, with one result.
func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request, c *call) {
	if _, err := flagParam(r.URL.Query(), "dryRun"); err != nil {
		writeErr(w, err)
		return
	}
	obj, err := objectParam(c.rt, r.URL.Query())
	if err != nil {
		writeClusterErr(w, err)
		return
	}
	var body editRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*maxManifestBytes)).Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeErr(w, errorf(http.StatusRequestEntityTooLarge, plugin.CodeBadRequest,
				"the manifests exceed %d MiB", 2*maxManifestBytes>>20))
			return
		}
		writeErr(w, badRequest("invalid JSON body: %v", err))
		return
	}
	dryRun := plugin.IsDryRun(r)
	if dryRun && obj.Resource.GroupResource() == secretsResource {
		var done func()
		w, done, err = s.secretDryRun(w, r, c, "POST /api/resource/edit")
		defer done()
		if err != nil {
			writeErr(w, err)
			return
		}
	}
	res, err := c.rt.EditObject(r.Context(), obj, []byte(body.Original), []byte(body.Edited), dryRun)
	var refused *kube.EditError
	switch {
	case errors.As(err, &refused) && refused.Redacted:
		writeErr(w, errorf(http.StatusBadRequest, plugin.CodeRedactedSecret, "%s", refused.Problem))
		return
	case errors.As(err, &refused):
		writeErr(w, badRequest("%s", refused.Problem))
		return
	case err != nil:
		writeErr(w, err)
		return
	}
	c.setAudit("", applyAuditDetail([]kube.ApplyResult{res}, false, dryRun))
	plugin.WriteJSON(w, map[string][]applyResult{"results": {toApplyResult(res)}})
}

func writeManifestErr(w http.ResponseWriter, err error) {
	var me *kube.ManifestError
	if errors.As(err, &me) && me.Redacted {
		writeErr(w, errorf(http.StatusBadRequest, plugin.CodeRedactedSecret, "%s", me.Error()))
		return
	}
	writeErr(w, badRequest("%s", err.Error()))
}

func toApplyResult(res kube.ApplyResult) applyResult {
	out := applyResult{
		APIVersion: res.APIVersion, Kind: res.Kind, Namespace: res.Namespace, Name: res.Name,
		Action: string(res.Action), Warnings: res.Warnings,
	}
	if res.Err != nil {
		out.Error, out.Code = res.Err.Error(), applyErrorCode(res.Err)
	}
	return out
}

// applyErrorCode is the §3.3 code of a failed object: conflict for a
// server-side apply conflict (the UI then offers to force the apply),
// not_found for a kind the cluster does not serve, kubernetes otherwise.
func applyErrorCode(err error) string {
	switch {
	case kube.IsApplyConflict(err):
		return plugin.CodeConflict
	case meta.IsNoMatchError(err):
		return plugin.CodeNotFound
	}
	return plugin.CodeKubernetes
}

// applyAuditTarget names the first object and how many followed it, e.g.
// "deployment/shop/web (+2 more)".
func applyAuditTarget(results []kube.ApplyResult) string {
	first := results[0]
	target := joinTarget(strings.ToLower(first.Kind), first.Namespace, first.Name)
	if more := len(results) - 1; more > 0 {
		target += fmt.Sprintf(" (+%d more)", more)
	}
	return target
}

// touchesSecrets reports whether any of objects is a core Secret.
func touchesSecrets(rt *kube.Runtime, objects []*unstructured.Unstructured) bool {
	for _, obj := range objects {
		if gvr, _, err := rt.ResolveKind(obj.GetAPIVersion(), obj.GetKind()); err == nil && gvr.GroupResource() == secretsResource {
			return true
		}
	}
	return false
}

// secretDryRun treats a dry run that touches a Secret like the change
// itself: it needs write access and is audited. A dry run tells whether
// the object would change, so a viewer, who sees Secret values only
// redacted, could otherwise confirm guessed values one unaudited dry run
// at a time (an "unchanged" result means the guess is right). The handler
// answers through the returned writer and calls done when it returns; err
// is the denial, which the row records once the handler writes it.
func (s *Server) secretDryRun(w http.ResponseWriter, r *http.Request, c *call, action string) (_ http.ResponseWriter, done func(), err error) {
	aw := s.newAuditWriter(w, r, c, action, auditTarget{kind: "resource", kindParam: true})
	c.audit = aw
	return aw, aw.finish, mutationDenied(c.role, c.cluster)
}

// applyAuditDetail counts the outcomes, e.g. {"configured":1,"failed":1},
// and flags forced applies and (audited) dry runs.
func applyAuditDetail(results []kube.ApplyResult, force, dryRun bool) string {
	detail := map[string]any{}
	for _, res := range results {
		outcome := string(res.Action)
		if res.Err != nil {
			outcome = "failed"
		}
		count, _ := detail[outcome].(int)
		detail[outcome] = count + 1
	}
	if force {
		detail["force"] = true
	}
	if dryRun {
		detail["dryRun"] = true
	}
	return auditJSON(detail)
}

// handleDeleteResource deletes one object of any kind with the chosen
// propagation policy and grace period.
func (s *Server) handleDeleteResource(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	opts, err := deleteOptions(q)
	if err != nil {
		writeErr(w, err)
		return
	}
	obj, err := objectParam(c.rt, q)
	if err != nil {
		writeClusterErr(w, err)
		return
	}
	c.setAudit(obj.Target(), "")
	if obj.Resource.GroupResource() == namespacesResource {
		writeErr(w, badRequest("Namespaces are deleted with DELETE /api/namespaces, which protects the system namespaces"))
		return
	}
	respondOK(w, c.rt.DeleteObject(r.Context(), obj, opts))
}

// deleteOptions parses propagation (Background, Foreground, Orphan) and
// gracePeriod (seconds), both optional.
func deleteOptions(q url.Values) (kube.DeleteOptions, error) {
	var opts kube.DeleteOptions
	switch p := metav1.DeletionPropagation(q.Get("propagation")); p {
	case "", metav1.DeletePropagationBackground, metav1.DeletePropagationForeground, metav1.DeletePropagationOrphan:
		opts.Propagation = p
	default:
		return opts, badRequest("propagation must be Background, Foreground or Orphan")
	}
	if raw := q.Get("gracePeriod"); raw != "" {
		seconds, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || seconds < 0 {
			return opts, badRequest("gracePeriod must be a number of seconds of at least 0")
		}
		opts.GracePeriod = &seconds
	}
	return opts, nil
}

// handlePatchMetadata sets or removes labels and annotations: the body is
// {"labels":{…},"annotations":{…}}, a null value removes the key.
func (s *Server) handlePatchMetadata(w http.ResponseWriter, r *http.Request, c *call) {
	obj, err := objectParam(c.rt, r.URL.Query())
	if err != nil {
		writeClusterErr(w, err)
		return
	}
	var patch kube.MetadataPatch
	if err := decodeJSON(w, r, &patch); err != nil {
		writeErr(w, err)
		return
	}
	if len(patch.Labels) == 0 && len(patch.Annotations) == 0 {
		writeErr(w, badRequest("nothing to change: name labels or annotations"))
		return
	}
	// Values are not recorded: annotations may hold anything.
	c.setAudit(obj.Target(), auditJSON(changedKeys(patch)))
	if obj.Resource.GroupResource() == secretsResource && holdsRedacted(patch) {
		writeErr(w, errorf(http.StatusBadRequest, plugin.CodeRedactedSecret,
			"a value is a redaction placeholder; it would replace the real value of the Secret's annotation"))
		return
	}
	respondOK(w, c.rt.PatchMetadata(r.Context(), obj, patch))
}

var (
	namespacesResource = schema.GroupResource{Resource: "namespaces"}
	secretsResource    = schema.GroupResource{Resource: "secrets"}
)

func holdsRedacted(patch kube.MetadataPatch) bool {
	for _, values := range []map[string]*string{patch.Labels, patch.Annotations} {
		for _, v := range values {
			if v != nil && kube.IsRedacted(*v) {
				return true
			}
		}
	}
	return false
}

// changedKeys lists the label and annotation keys a patch sets or removes.
func changedKeys(patch kube.MetadataPatch) map[string][]string {
	out := map[string][]string{}
	for field, values := range map[string]map[string]*string{"labels": patch.Labels, "annotations": patch.Annotations} {
		for key := range values {
			out[field] = append(out[field], key)
		}
		slices.Sort(out[field])
	}
	return out
}

// handleRevealSecret returns a Secret with its values. It changes nothing,
// but exposes what every other route redacts: the route needs exec access
// and, like every exec request, is audited (target secret/<ns>/<name>).
func (s *Server) handleRevealSecret(w http.ResponseWriter, r *http.Request, c *call) {
	ns, name, err := objectParams(r.URL.Query())
	if err != nil {
		writeErr(w, err)
		return
	}
	secret, err := c.rt.RevealSecret(r.Context(), ns, name)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	plugin.WriteJSON(w, secret)
}

// objectParam resolves the object a generic endpoint works on, named by
// apiVersion and kind or by a generic catalog key, plus ns (namespaced
// kinds only) and name.
func objectParam(rt *kube.Runtime, q url.Values) (kube.Object, error) {
	apiVersion, kind, name := q.Get("apiVersion"), q.Get("kind"), q.Get("name")
	if key := q.Get("key"); key != "" {
		if apiVersion != "" || kind != "" {
			return kube.Object{}, badRequest("name the resource by key or by apiVersion and kind, not both")
		}
		entry, ok := kube.Catalog[key]
		if !ok {
			return kube.Object{}, badRequest("unknown resource %q", key)
		}
		apiVersion, kind = entry.GVR.GroupVersion().String(), entry.Kind
	}
	if apiVersion == "" || kind == "" || name == "" {
		return kube.Object{}, badRequest("apiVersion, kind (or key) and name are required")
	}
	return rt.ResolveObject(apiVersion, kind, q.Get("ns"), name)
}

// writeClusterErr reports the error of a request on the active cluster:
// KubeBench's own refusals as 400 bad_request, errors from the cluster
// (including kinds it does not serve) through plugin.WriteK8sError.
func writeClusterErr(w http.ResponseWriter, err error) {
	var ae *apiError
	var re *kube.RequestError
	switch {
	case errors.As(err, &ae):
		writeErr(w, ae)
	case errors.As(err, &re):
		writeErr(w, badRequest("%s", re.Reason))
	default:
		plugin.WriteK8sError(w, err)
	}
}

// flagParam reads an optional boolean query flag. Only "true" and "false"
// are accepted, the spelling plugin.IsDryRun relies on, so that a flag
// never means one thing to the router and another to the handler.
func flagParam(q url.Values, key string) (bool, error) {
	switch q.Get(key) {
	case "", "false":
		return false, nil
	case "true":
		return true, nil
	}
	return false, badRequest("%s must be true or false", key)
}
