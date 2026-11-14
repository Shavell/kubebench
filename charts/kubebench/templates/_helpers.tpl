{{/* Chart name, overridable with nameOverride. */}}
{{- define "kubebench.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified app name: fullnameOverride, else "<release>-<name>", or the
release name alone when it already contains the chart name.
*/}}
{{- define "kubebench.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* The version that is actually deployed (image tag), usable as a label value. */}}
{{- define "kubebench.version" -}}
{{- default .Chart.AppVersion .Values.image.tag | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kubebench.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubebench.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "kubebench.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "kubebench.selectorLabels" . }}
app.kubernetes.io/version: {{ include "kubebench.version" . | quote }}
app.kubernetes.io/part-of: kubebench
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "kubebench.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "kubebench.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/* repository@digest when a digest is set, else repository:tag. */}}
{{- define "kubebench.image" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}
{{- end -}}

{{/* Name of the Secret holding the bootstrap admin password ("" = none). */}}
{{- define "kubebench.secretName" -}}
{{- if .Values.auth.existingSecret -}}
{{- .Values.auth.existingSecret -}}
{{- else if .Values.auth.createSecret -}}
{{- printf "%s-auth" (include "kubebench.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "kubebench.secretKey" -}}
{{- if .Values.auth.existingSecret -}}
{{- .Values.auth.existingSecretKey -}}
{{- else -}}
admin-password
{{- end -}}
{{- end -}}

{{/*
Bootstrap admin password for the chart-managed Secret: auth.adminPassword, else
the value already stored in the Secret (so upgrades keep it), else a new
random one. `lookup` is empty under `helm template`, so a render outside a
cluster always gets a fresh random value.
*/}}
{{- define "kubebench.adminPassword" -}}
{{- if .Values.auth.adminPassword -}}
{{- .Values.auth.adminPassword -}}
{{- else -}}
{{- $secret := lookup "v1" "Secret" .Release.Namespace (include "kubebench.secretName" .) -}}
{{- $stored := dig "data" "admin-password" "" ($secret | default dict) -}}
{{- if $stored -}}
{{- $stored | b64dec -}}
{{- else -}}
{{- randAlphaNum 24 -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* KUBEBENCH_TRUST_PROXY: config.trustProxy, or whether an Ingress fronts the pod. */}}
{{- define "kubebench.trustProxy" -}}
{{- if kindIs "invalid" .Values.config.trustProxy -}}
{{- .Values.ingress.enabled -}}
{{- else -}}
{{- .Values.config.trustProxy -}}
{{- end -}}
{{- end -}}

{{/* Name of the PVC mounted on /data. */}}
{{- define "kubebench.claimName" -}}
{{- default (printf "%s-data" (include "kubebench.fullname" .)) .Values.persistence.existingClaim -}}
{{- end -}}

{{/* Fails rendering on value combinations the schema cannot express. */}}
{{- define "kubebench.validate" -}}
{{- if and .Values.auth.adminPassword (not .Values.auth.createSecret) (not .Values.auth.existingSecret) -}}
{{- fail "auth.adminPassword needs auth.createSecret=true (or use auth.existingSecret)" -}}
{{- end -}}
{{- if and .Values.rbac.create (eq .Values.rbac.preset "custom") (not .Values.rbac.rules) -}}
{{- fail "rbac.preset=custom needs at least one entry in rbac.rules" -}}
{{- end -}}
{{- if and .Values.ingress.enabled (not .Values.ingress.hosts) -}}
{{- fail "ingress.enabled needs at least one entry in ingress.hosts" -}}
{{- end -}}
{{- end -}}
