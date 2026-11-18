package security

import "kubebench/internal/reason"

// Reasons of the security plugin: its refusals, skipped checks and report
// warnings (docs/design-v0.5.md §3.3).
var (
	reasonCannotList        = reason.Define("security.cannot_list", "cannot list {{kind}}: {{detail}}")
	reasonNotListed         = reason.Define("security.not_listed", "the objects these checks need could not be listed")
	reasonTruncatedKind     = reason.Define("security.truncated_kind", "only the first {{max}} {{kind}} were scanned")
	reasonTruncatedFindings = reason.Define("security.truncated_findings", "the report lists the first {{max}} findings only")
	reasonNoReport          = reason.Define("security.no_report", "no scan has run on this cluster yet")
	reasonBusy              = reason.Define("security.busy", "a scan with other options is already running on this cluster; try again when it has finished")
	reasonKeyLength         = reason.Define("security.key_too_long", "key: at most {{max}} bytes")
	reasonKeyFormat         = reason.Define("security.key_format", "key: expected <checkId>:<Kind>:<namespace>:<name>")
	reasonKeyCheck          = reason.Define("security.key_unknown_check", "key: unknown check \"{{check}}\"")
	reasonKeyKind           = reason.Define("security.key_unknown_kind", "key: unknown kind \"{{kind}}\"")
	reasonExpiresPast       = reason.Define("security.expires_in_past", "expiresAt: must be in the future")
	reasonNoAccepted        = reason.Define("security.no_accepted_risk", "no accepted risk \"{{key}}\"")
)
