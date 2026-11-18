package security

import (
	"time"

	"kubebench/internal/reason"
)

// severityWeight is how much one (check, object) pair of a severity counts
// in a score: a critical failure weighs as much as ten low ones.
var severityWeight = map[string]int{SeverityCritical: 10, SeverityHigh: 6, SeverityMedium: 3, SeverityLow: 1}

// buildReport applies the accepted risks valid at now to res and computes
// counts and scores. Accepted findings count as passing.
func buildReport(res *result, accepted map[string]Acceptance, now time.Time) *Report {
	rep := &Report{
		GeneratedAt: res.generatedAt, Cluster: res.cluster, Options: res.opts,
		Findings: make([]Finding, len(res.findings)), Skipped: res.skipped,
		Warnings: []string{}, WarningReasons: []reason.Ref{},
		Inventory: res.inventory, SystemNamespaces: res.systemNamespaces,
	}
	if rep.Skipped == nil {
		rep.Skipped = []Skipped{}
	}
	for _, w := range res.warnings {
		rep.Warnings = append(rep.Warnings, w.Error())
		rep.WarningReasons = append(rep.WarningReasons, w.Ref())
	}
	failed := map[string]int{} // active findings per check
	byCategory := map[string]*CategoryScore{}
	for _, cat := range categories {
		byCategory[cat] = &CategoryScore{Category: cat, BySeverity: map[string]int{}}
		for _, sev := range severities {
			byCategory[cat].BySeverity[sev] = 0
		}
	}
	for i, f := range res.findings {
		if a, ok := accepted[f.Key]; ok && a.valid(now) {
			f.Accepted = &a
		}
		rep.Findings[i] = f
		cat := byCategory[f.Category]
		rep.Counts.Total++
		if f.Accepted != nil {
			rep.Counts.Accepted++
			cat.Accepted++
			continue
		}
		failed[f.CheckID]++
		cat.Failed++
		cat.BySeverity[f.Severity]++
		switch f.Severity {
		case SeverityCritical:
			rep.Counts.Critical++
		case SeverityHigh:
			rep.Counts.High++
		case SeverityMedium:
			rep.Counts.Medium++
		case SeverityLow:
			rep.Counts.Low++
		}
	}

	// Score: the severity-weighted share of (check, object) pairs that pass.
	var total, passed int
	catTotal, catPassed := map[string]int{}, map[string]int{}
	for _, c := range catalog {
		n, ran := res.evaluated[c.ID]
		if !ran {
			continue
		}
		byCategory[c.Category].Checks++
		byCategory[c.Category].Evaluated += n
		w := severityWeight[c.Severity]
		total += w * n
		passed += w * (n - failed[c.ID])
		catTotal[c.Category] += w * n
		catPassed[c.Category] += w * (n - failed[c.ID])
	}
	rep.Score.Overall = percent(passed, total)
	for _, cat := range categories {
		cs := byCategory[cat]
		if cat == CategoryOpenShift && cs.Checks == 0 {
			continue // not an OpenShift cluster
		}
		cs.Score = percent(catPassed[cat], catTotal[cat])
		rep.Score.ByCategory = append(rep.Score.ByCategory, *cs)
	}
	return rep
}

// percent is passed/total in whole percent, rounded down so that any
// failure keeps a score below 100; nil when nothing was evaluated.
func percent(passed, total int) *int {
	if total == 0 {
		return nil
	}
	p := passed * 100 / total
	return &p
}
