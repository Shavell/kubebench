package argocd

// JSON shapes of the plugin API, mirrored by src/plugins/argocd/data/types.ts.
// Fields Argo CD has not filled in (a new Application has no status yet)
// are empty rather than missing, so the UI can render any object.

// Destination is where an Application deploys to.
type Destination struct {
	Server    string `json:"server"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// Source is one source of an Application: a Git directory or a Helm chart.
type Source struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path"`
	Chart          string `json:"chart"`
	TargetRevision string `json:"targetRevision"`
	Ref            string `json:"ref"`
}

// Summary is one row of the applications list.
type Summary struct {
	Name           string      `json:"name"`
	Namespace      string      `json:"namespace"`
	Project        string      `json:"project"`
	Repo           string      `json:"repo"`
	Path           string      `json:"path"`
	Chart          string      `json:"chart"`
	TargetRevision string      `json:"targetRevision"`
	SourceCount    int         `json:"sourceCount"`
	Destination    Destination `json:"destination"`
	// Sync is Synced, OutOfSync or Unknown; Health is Healthy,
	// Progressing, Degraded, Suspended, Missing or Unknown.
	Sync   string `json:"sync"`
	Health string `json:"health"`
	// OperationPhase is the phase of the current or last operation
	// (Running, Terminating, Succeeded, Failed, Error), "" when none ran.
	OperationPhase string `json:"operationPhase"`
	// Operating is set while an operation is requested or running: a new
	// sync or rollback is refused with 409 until it ends.
	Operating    bool   `json:"operating"`
	LastSyncedAt string `json:"lastSyncedAt"`
	Revision     string `json:"revision"`
	AutoSync     bool   `json:"autoSync"`
	Deleting     bool   `json:"deleting"`
}

// SyncPolicy is the Application's sync policy.
type SyncPolicy struct {
	Automated   bool     `json:"automated"`
	Prune       bool     `json:"prune"`
	SelfHeal    bool     `json:"selfHeal"`
	SyncOptions []string `json:"syncOptions"`
}

// Resource is one managed resource from status.resources.
type Resource struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Status is the resource's sync status, Health its health (empty when
	// Argo CD keeps resource health in its resource tree only).
	Status          string `json:"status"`
	Health          string `json:"health"`
	HealthMessage   string `json:"healthMessage"`
	Hook            bool   `json:"hook"`
	RequiresPruning bool   `json:"requiresPruning"`
	// CatalogKey names the KubeBench resource list of the kind (e.g.
	// "configmaps"), "" when KubeBench has no generic view for it.
	CatalogKey string `json:"catalogKey"`
}

// Condition is an Application or ApplicationSet condition.
type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
}

// HistoryEntry is one deployment from status.history; rollbacks name its ID.
type HistoryEntry struct {
	ID              int64    `json:"id"`
	Revision        string   `json:"revision"`
	Revisions       []string `json:"revisions"`
	DeployedAt      string   `json:"deployedAt"`
	DeployStartedAt string   `json:"deployStartedAt"`
	Sources         []Source `json:"sources"`
	InitiatedBy     string   `json:"initiatedBy"`
	Automated       bool     `json:"automated"`
}

// ResultResource is one resource of an operation's sync result.
type ResultResource struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	HookPhase string `json:"hookPhase"`
	SyncPhase string `json:"syncPhase"`
}

// OperationState is status.operationState: the current or last operation.
type OperationState struct {
	Phase       string           `json:"phase"`
	Message     string           `json:"message"`
	StartedAt   string           `json:"startedAt"`
	FinishedAt  string           `json:"finishedAt"`
	InitiatedBy string           `json:"initiatedBy"`
	Automated   bool             `json:"automated"`
	Revision    string           `json:"revision"`
	DryRun      bool             `json:"dryRun"`
	Prune       bool             `json:"prune"`
	RetryCount  int64            `json:"retryCount"`
	Resources   []ResultResource `json:"resources"`
}

// Detail is one Application with everything the detail page shows.
type Detail struct {
	Summary
	Sources       []Source        `json:"sources"`
	SyncPolicy    SyncPolicy      `json:"syncPolicy"`
	HealthMessage string          `json:"healthMessage"`
	ReconciledAt  string          `json:"reconciledAt"`
	Resources     []Resource      `json:"resources"`
	Conditions    []Condition     `json:"conditions"`
	History       []HistoryEntry  `json:"history"`
	Operation     *OperationState `json:"operation"`
	Images        []string        `json:"images"`
	URLs          []string        `json:"urls"`
}

// Project is one AppProject.
type Project struct {
	Name         string        `json:"name"`
	Namespace    string        `json:"namespace"`
	Description  string        `json:"description"`
	SourceRepos  []string      `json:"sourceRepos"`
	Destinations []Destination `json:"destinations"`
	// ClusterResources lists the allowed cluster-scoped kinds as
	// "group/kind" (group "" is the core group).
	ClusterResources []string `json:"clusterResources"`
	Roles            []string `json:"roles"`
	SyncWindows      int      `json:"syncWindows"`
}

// AppSetApplication is one Application an ApplicationSet generated.
type AppSetApplication struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Health  string `json:"health"`
	Message string `json:"message"`
}

// ApplicationSet is one ApplicationSet.
type ApplicationSet struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	// Generators names each generator's type, e.g. "list" or "matrix".
	Generators   []string            `json:"generators"`
	TemplateName string              `json:"templateName"`
	Project      string              `json:"project"`
	Applications []AppSetApplication `json:"applications"`
	Conditions   []Condition         `json:"conditions"`
}
