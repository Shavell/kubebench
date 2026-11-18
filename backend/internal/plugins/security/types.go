package security

import (
	"time"

	"kubebench/internal/reason"
)

// Severities, from the most to the least serious. Their weights make up the
// posture score (see score.go).
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
)

// severities lists the severities in report order.
var severities = []string{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}

// Categories group the checks; the overview shows one small multiple each.
const (
	CategoryBaseline   = "pss-baseline"   // Pod Security Standards, baseline
	CategoryRestricted = "pss-restricted" // Pod Security Standards, restricted
	CategoryHygiene    = "hygiene"        // resources, probes, image tags
	CategoryRBAC       = "rbac"
	CategoryNetwork    = "network"
	CategorySecrets    = "secrets"
	CategoryNamespaces = "namespaces"
	// CategoryOpenShift holds the SCC checks; reports list it only for
	// clusters where they ran (§7.15).
	CategoryOpenShift = "openshift"
)

// categories lists the categories in report order.
var categories = []string{
	CategoryBaseline, CategoryRestricted, CategoryHygiene, CategoryRBAC,
	CategoryNetwork, CategorySecrets, CategoryNamespaces, CategoryOpenShift,
}

// Resource names the object a finding is about.
type Resource struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
}

// Finding is one check failing on one object. Its Key is stable across
// scans, so an accepted risk stays accepted while the object exists.
type Finding struct {
	Key        string   `json:"key"`
	CheckID    string   `json:"checkId"`
	Title      string   `json:"title"`
	Severity   string   `json:"severity"`
	Category   string   `json:"category"`
	Frameworks []string `json:"frameworks"`
	Resource   Resource `json:"resource"`
	// Message is the English text of the failure; Evidence the details it
	// lists (container names, fields, rules), which the UI shows as is.
	Message     string      `json:"message"`
	Evidence    []string    `json:"evidence,omitempty"`
	Remediation string      `json:"remediation"`
	Accepted    *Acceptance `json:"accepted,omitempty"`
}

// Acceptance records that a finding is a known, accepted risk. It is
// shared by every user of the cluster.
type Acceptance struct {
	By        string     `json:"by"`
	Reason    string     `json:"reason"`
	At        time.Time  `json:"at"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// ScanOptions are the choices a scan was run with.
type ScanOptions struct {
	// IncludeSystem also scans the system namespaces and the cluster's own
	// (bootstrap) RBAC objects, which are left out by default.
	IncludeSystem bool `json:"includeSystem"`
}

// Skipped is a check that could not run, e.g. because the stored
// credential may not list the objects it needs.
type Skipped struct {
	CheckID string `json:"checkId"`
	// Reason is the English explanation; ReasonCode and ReasonParams let
	// the UI translate it (§3.3).
	Reason       string        `json:"reason"`
	ReasonCode   string        `json:"reasonCode,omitempty"`
	ReasonParams reason.Params `json:"reasonParams,omitempty"`
}

// Report answers POST /scan and GET /report.
type Report struct {
	GeneratedAt time.Time   `json:"generatedAt"`
	Cluster     string      `json:"cluster"`
	Options     ScanOptions `json:"options"`
	Score       Score       `json:"score"`
	Counts      Counts      `json:"counts"`
	Findings    []Finding   `json:"findings"`
	Skipped     []Skipped   `json:"skipped"`
	// Warnings are partial failures that did not stop any check, e.g. a
	// kind that could not be listed or a list cut at the scan's limit.
	Warnings []string `json:"warnings"`
	// WarningReasons explain Warnings, index by index, for translation.
	WarningReasons []reason.Ref `json:"warningReasons"`
	// Inventory counts the objects the scan looked at, by kind.
	Inventory map[string]int `json:"inventory"`
	// SystemNamespaces are the namespaces left out unless IncludeSystem:
	// the cluster's kube-*, openshift and openshift-* namespaces and those
	// labelled openshift.io/run-level or openshift.io/cluster-monitoring.
	SystemNamespaces []string `json:"systemNamespaces"`
}

// Counts are the report's findings by severity. The severities count
// active findings only; Accepted counts the accepted ones, Total all.
type Counts struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Accepted int `json:"accepted"`
	Total    int `json:"total"`
}

// Score is the posture as a weighted pass ratio, 0–100 (see score.go).
// A score is null when nothing it covers was evaluated.
type Score struct {
	Overall    *int            `json:"overall"`
	ByCategory []CategoryScore `json:"byCategory"`
}

// CategoryScore is one category's score and findings.
type CategoryScore struct {
	Category string `json:"category"`
	Score    *int   `json:"score"`
	// Checks is the number of the category's checks that ran.
	Checks int `json:"checks"`
	// Evaluated counts (check, object) pairs, Failed the active findings
	// among them and Accepted the accepted ones.
	Evaluated  int            `json:"evaluated"`
	Failed     int            `json:"failed"`
	Accepted   int            `json:"accepted"`
	BySeverity map[string]int `json:"bySeverity"`
}
