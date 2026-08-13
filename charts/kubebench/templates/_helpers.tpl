{{- define "kubebench.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kubebench.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "kubebench.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "kubebench.labels" -}}
app.kubernetes.io/name: {{ include "kubebench.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "kubebench.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubebench.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "kubebench.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "kubebench.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "kubebench.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}

{{- define "kubebench.secretName" -}}
{{- default (printf "%s-auth" (include "kubebench.fullname" .)) .Values.auth.existingSecret -}}
{{- end -}}
