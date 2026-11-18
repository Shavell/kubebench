{{/* External identities (docs/design-v0.5.md §3.6): environment, Secret and CA bundle. */}}

{{/* Name of the Secret holding the OIDC client secret ("" = a public client). */}}
{{- define "kubebench.oidcSecretName" -}}
{{- with .Values.identity.oidc -}}
{{- if .existingSecret -}}
{{- .existingSecret -}}
{{- else if .clientSecret -}}
{{- printf "%s-oidc" (include "kubebench.fullname" $) -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "kubebench.oidcSecretKey" -}}
{{- if .Values.identity.oidc.existingSecret -}}
{{- .Values.identity.oidc.existingSecretKey -}}
{{- else -}}
client-secret
{{- end -}}
{{- end -}}

{{/* Mount path of the OIDC CA bundle ConfigMap. */}}
{{- define "kubebench.oidcCADir" -}}/etc/kubebench/oidc-ca{{- end -}}

{{/* Container environment for external identities (indented by the caller). */}}
{{- define "kubebench.identityEnv" -}}
{{- with .Values.config.publicUrl }}
- name: KUBEBENCH_PUBLIC_URL
  value: {{ . | quote }}
{{- end }}
{{- if .Values.identity.requireGroup }}
- name: KUBEBENCH_AUTH_REQUIRE_GROUP
  value: "true"
{{- end }}
{{- with .Values.identity.groupMappings }}
- name: KUBEBENCH_AUTH_GROUP_MAPPINGS
  value: {{ toJson . | quote }}
{{- end }}
{{- with .Values.identity.oidc }}
{{- if .enabled }}
{{- if or (not .issuer) (not .clientId) }}
{{- fail "identity.oidc.enabled needs identity.oidc.issuer and identity.oidc.clientId" }}
{{- end }}
- name: KUBEBENCH_OIDC_ISSUER
  value: {{ .issuer | quote }}
- name: KUBEBENCH_OIDC_CLIENT_ID
  value: {{ .clientId | quote }}
- name: KUBEBENCH_OIDC_NAME
  value: {{ .name | quote }}
- name: KUBEBENCH_OIDC_SCOPES
  value: {{ join " " .scopes | quote }}
- name: KUBEBENCH_OIDC_USERNAME_CLAIM
  value: {{ .usernameClaim | quote }}
- name: KUBEBENCH_OIDC_GROUPS_CLAIM
  value: {{ .groupsClaim | quote }}
{{- with .allowedDomains }}
- name: KUBEBENCH_OIDC_ALLOWED_DOMAINS
  value: {{ join "," . | quote }}
{{- end }}
{{- with include "kubebench.oidcSecretName" $ }}
- name: KUBEBENCH_OIDC_CLIENT_SECRET
  valueFrom:
    secretKeyRef:
      name: {{ . }}
      key: {{ include "kubebench.oidcSecretKey" $ }}
{{- end }}
{{- if .caBundle }}
- name: KUBEBENCH_OIDC_CA_FILE
  value: {{ printf "%s/ca.crt" (include "kubebench.oidcCADir" $) | quote }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}

{{/* Whether the OIDC CA bundle ConfigMap is mounted. */}}
{{- define "kubebench.oidcCA" -}}
{{- if and .Values.identity.oidc.enabled .Values.identity.oidc.caBundle -}}true{{- end -}}
{{- end -}}
