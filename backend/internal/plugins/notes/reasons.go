package notes

import "kubebench/internal/reason"

// Reasons of the notes plugin's own refusals (docs/design-v0.5.md §3.3).
// Validation reasons name the request field ({{field}}, filled in by
// invalidField).
var (
	reasonScope         = reason.Define("notes.scope", "{{field}}: must be all, personal or shared")
	reasonClusters      = reason.Define("notes.clusters", "{{field}}: must be current or all")
	reasonTerms         = reason.Define("notes.too_many_terms", "{{field}}: at most {{max}} search terms")
	reasonLimit         = reason.Define("notes.limit", "{{field}}: must be 1-{{max}}")
	reasonGlobalShared  = reason.Define("notes.global_shared", "{{field}}: only personal notes can be global")
	reasonGlobalObject  = reason.Define("notes.global_object", "{{field}}: a note on an object belongs to its cluster")
	reasonVersion       = reason.Define("notes.version_required", "{{field}}: must name the version the edit is based on")
	reasonRequestSize   = reason.Define("notes.request_too_large", "{{field}}: the request is larger than {{max}} bytes")
	reasonTitleEmpty    = reason.Define("notes.title_empty", "{{field}}: must not be empty")
	reasonTitleLength   = reason.Define("notes.title_too_long", "{{field}}: must be at most {{max}} characters")
	reasonTitleLine     = reason.Define("notes.title_line", "{{field}}: must be a single line of text")
	reasonBodySize      = reason.Define("notes.body_too_large", "{{field}}: must be at most {{max}} KiB")
	reasonBodyUTF8      = reason.Define("notes.body_utf8", "{{field}}: must be UTF-8 text")
	reasonTag           = reason.Define("notes.invalid_tag", "{{field}}: \"{{tag}}\" is not a tag: 1-40 letters, digits, '.', '_' or '-', starting with a letter or digit")
	reasonTooManyTags   = reason.Define("notes.too_many_tags", "{{field}}: at most {{max}} tags")
	reasonKind          = reason.Define("notes.invalid_kind", "{{field}}: \"{{value}}\" is not a Kubernetes kind")
	reasonNamespace     = reason.Define("notes.invalid_namespace", "{{field}}: \"{{value}}\" is not a namespace name")
	reasonObjectName    = reason.Define("notes.invalid_name", "{{field}}: \"{{value}}\" is not an object name")
	reasonAPIVersion    = reason.Define("notes.invalid_api_version", "{{field}}: \"{{value}}\" is not an API version")
	reasonNoteID        = reason.Define("notes.invalid_id", "{{field}}: \"{{value}}\" is not a note id")
	reasonNotFound      = reason.Define("notes.not_found", "note not found")
	reasonChanged       = reason.Define("notes.changed", "the note was changed since you opened it; reload it or overwrite the other change")
	reasonNoteLimit     = reason.Define("notes.limit_reached", "you have reached the limit of {{max}} notes; delete notes you no longer need")
	reasonDeleteShared  = reason.Define("notes.delete_shared", "only its author or an administrator can delete a shared note")
	reasonInvalidJSON   = reason.Define("notes.invalid_json", "{{field}}: invalid JSON: {{detail}}")
	reasonTrailingData  = reason.Define("notes.trailing_data", "{{field}}: unexpected data after the JSON object")
	reasonBooleanFormat = reason.Define("notes.boolean", "{{field}}: must be true or false")
)
