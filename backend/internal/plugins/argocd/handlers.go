package argocd

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"kubebench/internal/plugin"
)

// Argo CD's markers on an Application.
const (
	refreshAnnotation = "argocd.argoproj.io/refresh"
	// resourcesFinalizer makes the controller delete the managed resources
	// (foreground) before the Application goes; its /background variant
	// does the same in the background.
	resourcesFinalizer           = "resources-finalizer.argocd.argoproj.io"
	resourcesFinalizerBackground = resourcesFinalizer + "/background"
)

func applications(c *plugin.Context, ns string) dynamic.ResourceInterface {
	return c.Runtime.Dyn.Resource(applicationsGVR).Namespace(ns)
}

// listApplications serves GET /applications?ns= (all namespaces by default).
func listApplications(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	list, err := applications(c, r.URL.Query().Get("ns")).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]Summary, len(list.Items))
	for i := range list.Items {
		out[i] = toSummary(&list.Items[i])
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Namespace < b.Namespace
	})
	plugin.WriteJSON(w, out)
}

func getApplication(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	app, err := applications(c, r.PathValue("ns")).Get(r.Context(), r.PathValue("name"), metav1.GetOptions{})
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, toDetail(app))
}

// syncApplication serves POST …/sync: it requests a sync operation.
func syncApplication(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var req SyncRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	app, err := startOperation(r.Context(), c, r.PathValue("ns"), r.PathValue("name"), func(app map[string]any) (map[string]any, error) {
		return syncOperation(app, req, c.User.Username)
	})
	if err != nil {
		writeError(w, err)
		return
	}
	auditOperation(c, "argocd.sync", app, map[string]any{
		"revision": req.Revision, "prune": req.Prune, "dryRun": req.DryRun, "resources": len(req.Resources),
	})
	plugin.WriteJSON(w, toDetail(app))
}

// rollbackApplication serves POST …/rollback: a sync to a history entry.
func rollbackApplication(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	var req RollbackRequest
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	app, err := startOperation(r.Context(), c, r.PathValue("ns"), r.PathValue("name"), func(app map[string]any) (map[string]any, error) {
		return rollbackOperation(app, req, c.User.Username)
	})
	if err != nil {
		writeError(w, err)
		return
	}
	auditOperation(c, "argocd.rollback", app, map[string]any{"historyId": *req.HistoryID, "prune": req.Prune})
	plugin.WriteJSON(w, toDetail(app))
}

// startOperation sets the Application's operation built by build, unless
// one is already requested or running (409). The patch carries the
// resourceVersion that was checked, so an operation the controller
// started meanwhile makes it fail with a 409 instead of being overwritten.
func startOperation(ctx context.Context, c *plugin.Context, ns, name string,
	build func(app map[string]any) (map[string]any, error)) (*unstructured.Unstructured, error) {
	res := applications(c, ns)
	app, err := res.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if operating(app.Object) {
		return nil, conflict("another operation is already running on application %s/%s; wait for it to finish", ns, name)
	}
	op, err := build(app.Object)
	if err != nil {
		return nil, err
	}
	return patch(ctx, res, app, map[string]any{"operation": op}, nil)
}

// patch merge-patches app with body plus, under metadata, the given fields
// and app's resourceVersion (an optimistic lock).
func patch(ctx context.Context, res dynamic.ResourceInterface, app *unstructured.Unstructured,
	body, metadata map[string]any) (*unstructured.Unstructured, error) {
	meta := map[string]any{"resourceVersion": app.GetResourceVersion()}
	for k, v := range metadata {
		meta[k] = v
	}
	body["metadata"] = meta
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return res.Patch(ctx, app.GetName(), types.MergePatchType, data, metav1.PatchOptions{})
}

// auditOperation adds a row naming what the operation asked for: the
// router's row for the request cannot see the body.
func auditOperation(c *plugin.Context, action string, app *unstructured.Unstructured, detail map[string]any) {
	data, err := json.Marshal(detail)
	if err != nil {
		return
	}
	c.Audit(action, "application/"+app.GetNamespace()+"/"+app.GetName(), string(data))
}

// refreshApplication serves POST …/refresh?hard=: it asks the controller
// to compare the Application with its source again, a hard refresh also
// regenerating the manifests instead of using the repo server's cache.
func refreshApplication(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	hard, err := boolParam(r, "hard", false)
	if err != nil {
		writeError(w, err)
		return
	}
	level := "normal"
	if hard {
		level = "hard"
	}
	res := applications(c, r.PathValue("ns"))
	app, err := res.Get(r.Context(), r.PathValue("name"), metav1.GetOptions{})
	if err != nil {
		writeError(w, err)
		return
	}
	updated, err := patch(r.Context(), res, app, map[string]any{}, map[string]any{
		"annotations": map[string]any{refreshAnnotation: level},
	})
	if err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, toDetail(updated))
}

// deleteApplication serves DELETE …?cascade= (default true, as in Argo
// CD). Cascading adds the resources finalizer, so the controller deletes
// the managed resources first; otherwise the finalizer is removed and the
// resources stay.
func deleteApplication(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	cascade, err := boolParam(r, "cascade", true)
	if err != nil {
		writeError(w, err)
		return
	}
	ns, name := r.PathValue("ns"), r.PathValue("name")
	res := applications(c, ns)
	app, err := res.Get(r.Context(), name, metav1.GetOptions{})
	if err != nil {
		writeError(w, err)
		return
	}
	finalizers, changed := cascadeFinalizers(app.GetFinalizers(), cascade)
	if changed {
		if cascade && app.GetDeletionTimestamp() != nil {
			writeError(w, conflict("application %s/%s is already being deleted without cascade", ns, name))
			return
		}
		if app, err = patch(r.Context(), res, app, map[string]any{}, map[string]any{"finalizers": toAnyList(finalizers)}); err != nil {
			writeError(w, err)
			return
		}
	}
	uid := app.GetUID()
	if err := res.Delete(r.Context(), name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil {
		writeError(w, err)
		return
	}
	plugin.WriteJSON(w, map[string]bool{"deleted": true, "cascade": cascade})
}

// cascadeFinalizers returns the finalizers an Application needs for the
// chosen deletion and whether they differ from current.
func cascadeFinalizers(current []string, cascade bool) ([]string, bool) {
	isResources := func(f string) bool { return f == resourcesFinalizer || f == resourcesFinalizerBackground }
	has := slices.ContainsFunc(current, isResources)
	switch {
	case cascade && !has:
		return append(slices.Clone(current), resourcesFinalizer), true
	case !cascade && has:
		return slices.DeleteFunc(slices.Clone(current), isResources), true
	}
	return current, false
}

func listProjects(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	list, err := c.Runtime.Dyn.Resource(projectsGVR).Namespace(r.URL.Query().Get("ns")).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]Project, len(list.Items))
	for i := range list.Items {
		out[i] = toProject(&list.Items[i])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Namespace+"/"+out[i].Name < out[j].Namespace+"/"+out[j].Name })
	plugin.WriteJSON(w, out)
}

func listApplicationSets(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	list, err := c.Runtime.Dyn.Resource(applicationSetsGVR).Namespace(r.URL.Query().Get("ns")).List(r.Context(), metav1.ListOptions{})
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]ApplicationSet, len(list.Items))
	for i := range list.Items {
		out[i] = toApplicationSet(&list.Items[i])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Namespace+"/"+out[i].Name < out[j].Namespace+"/"+out[j].Name })
	plugin.WriteJSON(w, out)
}
