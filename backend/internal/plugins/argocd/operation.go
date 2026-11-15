package argocd

import "slices"

// Building the Application .operation for syncs and rollbacks, as the Argo
// CD API server does (server/application: Sync and Rollback). The
// controller acts only on what the operation says: it does not fall back
// to the Application's sync options, so they are copied in here unless the
// caller names its own.

// SyncRequest is the body of POST /applications/{ns}/{name}/sync.
type SyncRequest struct {
	// Revision to sync to; empty syncs the source's targetRevision.
	Revision  string        `json:"revision"`
	Prune     bool          `json:"prune"`
	DryRun    bool          `json:"dryRun"`
	Resources []ResourceKey `json:"resources"`
	// SyncOptions replace the Application's spec.syncPolicy.syncOptions
	// for this sync; omitted (null) keeps them, [] syncs without any.
	SyncOptions *[]string `json:"syncOptions"`
}

// ResourceKey names one resource of a selective sync.
type ResourceKey struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// RollbackRequest is the body of POST /applications/{ns}/{name}/rollback.
type RollbackRequest struct {
	// HistoryID is the id of the status.history entry to go back to.
	HistoryID *int64 `json:"historyId"`
	Prune     bool   `json:"prune"`
}

// initiator is operation.initiatedBy for a KubeBench user (§7.5).
func initiator(username string) map[string]any {
	return map[string]any{"username": "kubebench:" + username}
}

// syncOperation builds the operation for a sync request on app.
func syncOperation(app map[string]any, req SyncRequest, username string) (map[string]any, error) {
	spec := object(app, "spec")
	sync := map[string]any{}
	if multi := objects(spec, "sources"); len(multi) > 0 {
		// A multi-source Application syncs every source at its own
		// targetRevision; one revision cannot name them all.
		if req.Revision != "" {
			return nil, badRequest("a revision cannot be chosen for an application with several sources")
		}
		revisions := make([]any, len(multi))
		for i, s := range multi {
			revisions[i] = str(s, "targetRevision")
		}
		sync["revisions"] = revisions
	} else if revision := firstNonEmpty(req.Revision, str(spec, "source", "targetRevision")); revision != "" {
		sync["revision"] = revision
	}
	if req.Prune {
		sync["prune"] = true
	}
	if req.DryRun {
		sync["dryRun"] = true
	}
	if len(req.Resources) > 0 {
		resources := make([]any, len(req.Resources))
		for i, r := range req.Resources {
			if r.Kind == "" || r.Name == "" {
				return nil, badRequest("every selected resource needs a kind and a name")
			}
			resources[i] = map[string]any{"group": r.Group, "kind": r.Kind, "namespace": r.Namespace, "name": r.Name}
		}
		sync["resources"] = resources
	}
	options := stringList(spec, "syncPolicy", "syncOptions")
	if req.SyncOptions != nil {
		options = *req.SyncOptions
	}
	if len(options) > 0 {
		sync["syncOptions"] = toAnyList(options)
	}
	return map[string]any{"initiatedBy": initiator(username), "sync": sync}, nil
}

// rollbackOperation builds the operation that syncs app back to the
// revision and sources of one history entry. Like Argo CD, it refuses
// while automated sync is on (the controller would sync forward again)
// and uses the apply strategy, so hooks do not run again.
func rollbackOperation(app map[string]any, req RollbackRequest, username string) (map[string]any, error) {
	spec := object(app, "spec")
	if autoSyncEnabled(spec) {
		return nil, conflict("rollback is not possible while automated sync is enabled; disable it first")
	}
	if req.HistoryID == nil {
		return nil, badRequest("historyId is required")
	}
	history := objects(app, "status", "history")
	i := slices.IndexFunc(history, func(h map[string]any) bool {
		id, ok := integer(h, "id")
		return ok && id == *req.HistoryID
	})
	if i < 0 {
		return nil, notFound("the application has no history entry %d", *req.HistoryID)
	}
	entry := history[i]
	sync := map[string]any{"syncStrategy": map[string]any{"apply": map[string]any{}}}
	if multi := objects(entry, "sources"); len(multi) > 0 {
		sync["sources"] = toAnyMaps(multi)
		sync["revisions"] = toAnyList(stringList(entry, "revisions"))
	} else if source := object(entry, "source"); len(source) > 0 {
		sync["source"] = source
		sync["revision"] = str(entry, "revision")
	} else {
		return nil, badRequest("history entry %d does not record its source (deployed by Argo CD 1.4 or older)", *req.HistoryID)
	}
	if req.Prune {
		sync["prune"] = true
	}
	if options := stringList(spec, "syncPolicy", "syncOptions"); len(options) > 0 {
		sync["syncOptions"] = toAnyList(options)
	}
	return map[string]any{"initiatedBy": initiator(username), "sync": sync}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// toAnyList and toAnyMaps convert to the []any that unstructured objects
// and JSON merge patches hold.
func toAnyList(items []string) []any {
	out := make([]any, len(items))
	for i, s := range items {
		out[i] = s
	}
	return out
}

func toAnyMaps(items []map[string]any) []any {
	out := make([]any, len(items))
	for i, m := range items {
		out[i] = m
	}
	return out
}
