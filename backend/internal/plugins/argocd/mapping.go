package argocd

import (
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"kubebench/internal/kube"
)

// Mapping of Argo CD objects to the API shapes in types.go (§7.5). Status
// fields are absent until the controller has reconciled an Application, so
// every reader falls back to the zero value or Argo CD's "Unknown".

const unknown = "Unknown"

// sources returns the Application's sources: spec.sources of a
// multi-source Application, else spec.source.
func sources(spec map[string]any) []Source {
	if multi := objects(spec, "sources"); len(multi) > 0 {
		out := make([]Source, len(multi))
		for i, s := range multi {
			out[i] = toSource(s)
		}
		return out
	}
	if single := object(spec, "source"); single != nil {
		return []Source{toSource(single)}
	}
	return []Source{}
}

func toSource(s map[string]any) Source {
	return Source{
		RepoURL:        str(s, "repoURL"),
		Path:           str(s, "path"),
		Chart:          str(s, "chart"),
		TargetRevision: str(s, "targetRevision"),
		Ref:            str(s, "ref"),
	}
}

// autoSyncEnabled reports whether automated sync is on. Argo CD 3 lets an
// automated block be switched off with enabled: false.
func autoSyncEnabled(spec map[string]any) bool {
	automated, ok := field(spec, "syncPolicy", "automated").(map[string]any)
	if !ok {
		return false
	}
	enabled, set := automated["enabled"].(bool)
	return !set || enabled
}

// operating reports whether an operation is requested or still running.
// The controller clears .operation once it picks the request up and
// reports progress in status.operationState.
func operating(app map[string]any) bool {
	if object(app, "operation") != nil {
		return true
	}
	switch str(app, "status", "operationState", "phase") {
	case "Running", "Terminating":
		return true
	}
	return false
}

func toSummary(u *unstructured.Unstructured) Summary {
	app := u.Object
	spec := object(app, "spec")
	status := object(app, "status")
	srcs := sources(spec)
	s := Summary{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
		Project:   str(spec, "project"),
		Destination: Destination{
			Server:    str(spec, "destination", "server"),
			Name:      str(spec, "destination", "name"),
			Namespace: str(spec, "destination", "namespace"),
		},
		SourceCount:    len(srcs),
		Sync:           orUnknown(str(status, "sync", "status")),
		Health:         orUnknown(str(status, "health", "status")),
		OperationPhase: str(status, "operationState", "phase"),
		Operating:      operating(app),
		Revision:       str(status, "sync", "revision"),
		AutoSync:       autoSyncEnabled(spec),
		Deleting:       u.GetDeletionTimestamp() != nil,
	}
	if s.Project == "" {
		s.Project = "default" // Argo CD's own default for an empty project
	}
	if len(srcs) > 0 {
		s.Repo, s.Path, s.Chart, s.TargetRevision = srcs[0].RepoURL, srcs[0].Path, srcs[0].Chart, srcs[0].TargetRevision
	}
	if s.Revision == "" {
		if revs := stringList(status, "sync", "revisions"); len(revs) > 0 {
			s.Revision = revs[0]
		}
	}
	s.LastSyncedAt = lastSyncedAt(status)
	return s
}

func orUnknown(s string) string {
	if s == "" {
		return unknown
	}
	return s
}

// lastSyncedAt is when the newest history entry was deployed, else when
// the last operation finished.
func lastSyncedAt(status map[string]any) string {
	if history := objects(status, "history"); len(history) > 0 {
		if at := str(history[len(history)-1], "deployedAt"); at != "" {
			return at
		}
	}
	return str(status, "operationState", "finishedAt")
}

func toDetail(u *unstructured.Unstructured) Detail {
	app := u.Object
	spec := object(app, "spec")
	status := object(app, "status")
	return Detail{
		Summary: toSummary(u),
		Sources: sources(spec),
		SyncPolicy: SyncPolicy{
			Automated:   autoSyncEnabled(spec),
			Prune:       boolean(spec, "syncPolicy", "automated", "prune"),
			SelfHeal:    boolean(spec, "syncPolicy", "automated", "selfHeal"),
			SyncOptions: stringList(spec, "syncPolicy", "syncOptions"),
		},
		HealthMessage: str(status, "health", "message"),
		ReconciledAt:  str(status, "reconciledAt"),
		Resources:     toResources(objects(status, "resources")),
		Conditions:    toConditions(objects(status, "conditions")),
		History:       toHistory(objects(status, "history")),
		Operation:     toOperationState(object(status, "operationState")),
		Images:        stringList(status, "summary", "images"),
		URLs:          stringList(status, "summary", "externalURLs"),
	}
}

func toResources(items []map[string]any) []Resource {
	out := make([]Resource, len(items))
	for i, r := range items {
		group, kind := str(r, "group"), str(r, "kind")
		out[i] = Resource{
			Group:           group,
			Version:         str(r, "version"),
			Kind:            kind,
			Namespace:       str(r, "namespace"),
			Name:            str(r, "name"),
			Status:          str(r, "status"),
			Health:          str(r, "health", "status"),
			HealthMessage:   str(r, "health", "message"),
			Hook:            boolean(r, "hook"),
			RequiresPruning: boolean(r, "requiresPruning"),
			CatalogKey:      catalogKeys[group+"/"+kind],
		}
	}
	return out
}

// catalogKeys maps "group/kind" to the key of KubeBench's generic
// resource list for that kind, so the UI can open managed resources there.
var catalogKeys = func() map[string]string {
	keys := make(map[string]string, len(kube.Catalog))
	for _, t := range kube.Catalog {
		keys[t.GVR.Group+"/"+t.Kind] = t.Key
	}
	return keys
}()

func toConditions(items []map[string]any) []Condition {
	out := make([]Condition, len(items))
	for i, c := range items {
		out[i] = Condition{
			Type:               str(c, "type"),
			Status:             str(c, "status"),
			Reason:             str(c, "reason"),
			Message:            str(c, "message"),
			LastTransitionTime: str(c, "lastTransitionTime"),
		}
	}
	return out
}

// toHistory returns the deployments newest first. Entries without an id
// cannot be rolled back to and are left out.
func toHistory(items []map[string]any) []HistoryEntry {
	out := make([]HistoryEntry, 0, len(items))
	for _, h := range items {
		id, ok := integer(h, "id")
		if !ok {
			continue
		}
		entry := HistoryEntry{
			ID:              id,
			Revision:        str(h, "revision"),
			Revisions:       stringList(h, "revisions"),
			DeployedAt:      str(h, "deployedAt"),
			DeployStartedAt: str(h, "deployStartedAt"),
			Sources:         sources(h),
			InitiatedBy:     str(h, "initiatedBy", "username"),
			Automated:       boolean(h, "initiatedBy", "automated"),
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

func toOperationState(op map[string]any) *OperationState {
	if op == nil {
		return nil
	}
	revision := str(op, "syncResult", "revision")
	if revision == "" {
		revision = str(op, "operation", "sync", "revision")
	}
	retries, _ := integer(op, "retryCount")
	results := objects(op, "syncResult", "resources")
	resources := make([]ResultResource, len(results))
	for i, r := range results {
		resources[i] = ResultResource{
			Group:     str(r, "group"),
			Kind:      str(r, "kind"),
			Namespace: str(r, "namespace"),
			Name:      str(r, "name"),
			Status:    str(r, "status"),
			Message:   str(r, "message"),
			HookPhase: str(r, "hookPhase"),
			SyncPhase: str(r, "syncPhase"),
		}
	}
	return &OperationState{
		Phase:       str(op, "phase"),
		Message:     str(op, "message"),
		StartedAt:   str(op, "startedAt"),
		FinishedAt:  str(op, "finishedAt"),
		InitiatedBy: str(op, "operation", "initiatedBy", "username"),
		Automated:   boolean(op, "operation", "initiatedBy", "automated"),
		Revision:    revision,
		DryRun:      boolean(op, "operation", "sync", "dryRun"),
		Prune:       boolean(op, "operation", "sync", "prune"),
		RetryCount:  retries,
		Resources:   resources,
	}
}

func toProject(u *unstructured.Unstructured) Project {
	spec := object(u.Object, "spec")
	dests := objects(spec, "destinations")
	p := Project{
		Name:             u.GetName(),
		Namespace:        u.GetNamespace(),
		Description:      str(spec, "description"),
		SourceRepos:      stringList(spec, "sourceRepos"),
		Destinations:     make([]Destination, len(dests)),
		ClusterResources: []string{},
		Roles:            []string{},
		SyncWindows:      len(objects(spec, "syncWindows")),
	}
	for i, d := range dests {
		p.Destinations[i] = Destination{Server: str(d, "server"), Name: str(d, "name"), Namespace: str(d, "namespace")}
	}
	for _, gk := range objects(spec, "clusterResourceWhitelist") {
		p.ClusterResources = append(p.ClusterResources, str(gk, "group")+"/"+str(gk, "kind"))
	}
	for _, role := range objects(spec, "roles") {
		p.Roles = append(p.Roles, str(role, "name"))
	}
	return p
}

func toApplicationSet(u *unstructured.Unstructured) ApplicationSet {
	spec := object(u.Object, "spec")
	status := object(u.Object, "status")
	set := ApplicationSet{
		Name:         u.GetName(),
		Namespace:    u.GetNamespace(),
		Generators:   []string{},
		TemplateName: str(spec, "template", "metadata", "name"),
		Project:      str(spec, "template", "spec", "project"),
		Conditions:   toConditions(objects(status, "conditions")),
	}
	for _, g := range objects(spec, "generators") {
		set.Generators = append(set.Generators, generatorTypes(g)...)
	}
	apps := objects(status, "resources")
	set.Applications = make([]AppSetApplication, len(apps))
	for i, a := range apps {
		set.Applications[i] = AppSetApplication{
			Name:    str(a, "name"),
			Status:  str(a, "status"),
			Health:  str(a, "health", "status"),
			Message: str(a, "health", "message"),
		}
	}
	return set
}

// generatorTypes names a generator by its key ("list", "git", "matrix",
// …), skipping the selector that may sit beside it.
func generatorTypes(g map[string]any) []string {
	out := make([]string, 0, 1)
	for key := range g {
		if key != "selector" {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}
