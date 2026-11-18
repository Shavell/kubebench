package access

import "kubebench/internal/reason"

// Reasons of the access plugin's own refusals (docs/design-v0.5.md §3.3).
// Validation reasons name the request field ({{field}}, filled in by
// invalidField).
var (
	reasonNonResourceFields = reason.Define("access.non_resource_fields", "{{field}}: a non-resource URL takes no group, resource, namespace or name")
	reasonPathSlash         = reason.Define("access.path_slash", "{{field}}: must start with /")
	reasonResourceRequired  = reason.Define("access.resource_required", "{{field}}: is required (or path, for a non-resource URL)")
	reasonNoSpaces          = reason.Define("access.no_spaces", "{{field}}: must not contain spaces or slashes")
	reasonOnlySANamespace   = reason.Define("access.namespace_only_service_account", "{{field}}: only a ServiceAccount has a namespace")
	reasonSubjectKind       = reason.Define("access.subject_kind", "{{field}}: must be User, Group or ServiceAccount, not \"{{kind}}\"")
	reasonReviewRate        = reason.Define("access.review_rate", "at most {{max}} access reviews per minute; try again in {{wait}}")
	reasonCannotReview      = reason.Define("access.cannot_review", "KubeBench's credential for this cluster may not create SubjectAccessReviews (it needs create on subjectaccessreviews.authorization.k8s.io cluster-wide): {{detail}}")
)
