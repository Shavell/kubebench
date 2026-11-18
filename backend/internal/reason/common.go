package reason

// Shared codes for request validation, used by the core API and the
// plugins alike. Field names are API identifiers and stay untranslated.
var (
	InvalidJSON  = Define("request.invalid_json", "invalid JSON body: {{detail}}")
	TrailingData = Define("request.trailing_data", "invalid JSON body: unexpected data after the object")
	// BodyTooLarge takes the limit with its unit, e.g. "256 KiB".
	BodyTooLarge = Define("request.too_large", "the request body is larger than {{size}}")

	FieldRequired = Define("field.required", "{{field}} is required")
	FieldBoolean  = Define("field.boolean", "{{field}} must be true or false")
	FieldRange    = Define("field.range", "{{field}} must be a number from {{min}} to {{max}}")
	FieldMin      = Define("field.min", "{{field}} must be a number of at least {{min}}")
	FieldTooLong  = Define("field.too_long", "{{field}} must be at most {{max}} characters")
	// FieldOneOf lists the allowed values, e.g. "off, stable, prerelease".
	FieldOneOf = Define("field.one_of", "{{field}} must be one of: {{values}}")
	// FieldInvalid carries a validation message of Kubernetes (English).
	FieldInvalid = Define("field.invalid", "{{field}}: {{detail}}")
	NotNamespace = Define("field.not_namespace", "\"{{value}}\" is not a namespace name")
	NotName      = Define("field.not_name", "\"{{value}}\" is not an object name")
)
