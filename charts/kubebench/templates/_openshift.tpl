{{/* OpenShift mode, the Route and the server's own TLS. */}}

{{/*
"true" in OpenShift mode, else "". openshift.enabled=auto looks for the
Route and SCC APIs; an offline `helm template` sees neither unless they are
passed with --api-versions.
*/}}
{{- define "kubebench.openshift" -}}
{{- $mode := toString .Values.openshift.enabled -}}
{{- if eq $mode "true" -}}
true
{{- else if and (eq $mode "auto") (.Capabilities.APIVersions.Has "route.openshift.io/v1") (.Capabilities.APIVersions.Has "security.openshift.io/v1") -}}
true
{{- end -}}
{{- end -}}

{{/* "true" when the chart renders a Route (route.enabled in OpenShift mode). */}}
{{- define "kubebench.route" -}}
{{- if and .Values.route.enabled (include "kubebench.openshift" .) -}}
true
{{- end -}}
{{- end -}}

{{/* "true" when the pod's certificate comes from the OpenShift service CA. */}}
{{- define "kubebench.serviceCA" -}}
{{- if and (include "kubebench.route" .) (eq .Values.route.tls.termination "reencrypt") (not .Values.tls.existingSecret) -}}
true
{{- end -}}
{{- end -}}

{{/* Secret with the server's TLS certificate and key ("" = plain HTTP). */}}
{{- define "kubebench.tlsSecretName" -}}
{{- if .Values.tls.existingSecret -}}
{{- .Values.tls.existingSecret -}}
{{- else if include "kubebench.serviceCA" . -}}
{{- printf "%s-tls" (include "kubebench.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/* Mount path of the server's TLS Secret. */}}
{{- define "kubebench.tlsDir" -}}/etc/kubebench/tls{{- end -}}

{{/*
Security contexts as YAML. In OpenShift mode the SCC assigns the user, the
group and fsGroup from the project's ranges and refuses fixed values
outside them, so they are left out.
*/}}
{{- define "kubebench.podSecurityContext" -}}
{{- if include "kubebench.openshift" . -}}
{{- toYaml (omit .Values.podSecurityContext "runAsUser" "runAsGroup" "fsGroup") -}}
{{- else -}}
{{- toYaml .Values.podSecurityContext -}}
{{- end -}}
{{- end -}}

{{- define "kubebench.containerSecurityContext" -}}
{{- if include "kubebench.openshift" . -}}
{{- toYaml (omit .Values.securityContext "runAsUser" "runAsGroup") -}}
{{- else -}}
{{- toYaml .Values.securityContext -}}
{{- end -}}
{{- end -}}

{{/* SCC the pod is pinned to in OpenShift mode. */}}
{{- define "kubebench.requiredSCC" -}}
{{- if .Values.openshift.requiredSCC -}}
{{- .Values.openshift.requiredSCC -}}
{{- else if eq (toString .Values.hostUsers) "false" -}}
restricted-v3
{{- else -}}
restricted-v2
{{- end -}}
{{- end -}}

{{/* Fails rendering on Route and TLS combinations the schema cannot express. */}}
{{- define "kubebench.validateOpenShift" -}}
{{- if and .Values.route.enabled (not (include "kubebench.openshift" .)) -}}
{{- fail "route.enabled needs OpenShift mode: openshift.enabled=true, or auto on a cluster serving route.openshift.io/v1 and security.openshift.io/v1 (an offline helm template sees no cluster APIs)" -}}
{{- end -}}
{{- with .Values.route.tls -}}
{{- if eq .termination "passthrough" -}}
{{- if not $.Values.tls.existingSecret -}}
{{- fail "route.tls.termination=passthrough needs tls.existingSecret: the pod serves the certificate for the Route host" -}}
{{- end -}}
{{- if $.Values.route.path -}}
{{- fail "route.path is not allowed with route.tls.termination=passthrough" -}}
{{- end -}}
{{- if eq .insecureEdgeTerminationPolicy "Allow" -}}
{{- fail "route.tls.insecureEdgeTerminationPolicy=Allow is not allowed with passthrough termination" -}}
{{- end -}}
{{- if or .certificate .key .caCertificate .destinationCACertificate -}}
{{- fail "route.tls certificates are not used with passthrough termination; the pod serves tls.existingSecret" -}}
{{- end -}}
{{- end -}}
{{- if and .destinationCACertificate (ne .termination "reencrypt") -}}
{{- fail "route.tls.destinationCACertificate needs route.tls.termination=reencrypt" -}}
{{- end -}}
{{- if ne (empty .certificate) (empty .key) -}}
{{- fail "route.tls.certificate and route.tls.key must be set together" -}}
{{- end -}}
{{- end -}}
{{- end -}}
