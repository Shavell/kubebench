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
{{- include "kubebench.ldapEnv" . }}
{{- end -}}

{{/* Name of the Secret holding the LDAP bind password ("" = anonymous search). */}}
{{- define "kubebench.ldapSecretName" -}}
{{- with .Values.identity.ldap -}}
{{- if .existingSecret -}}
{{- .existingSecret -}}
{{- else if .bindPassword -}}
{{- printf "%s-ldap" (include "kubebench.fullname" $) -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "kubebench.ldapSecretKey" -}}
{{- if .Values.identity.ldap.existingSecret -}}
{{- .Values.identity.ldap.existingSecretKey -}}
{{- else -}}
bind-password
{{- end -}}
{{- end -}}

{{/* Mount path of the LDAP CA bundle ConfigMap. */}}
{{- define "kubebench.ldapCADir" -}}/etc/kubebench/ldap-ca{{- end -}}

{{/* Whether the LDAP CA bundle ConfigMap is mounted. */}}
{{- define "kubebench.ldapCA" -}}
{{- if and .Values.identity.ldap.enabled .Values.identity.ldap.caBundle -}}true{{- end -}}
{{- end -}}

{{/* KUBEBENCH_LDAP_* for the directory provider; optional values only when set. */}}
{{- define "kubebench.ldapEnv" -}}
{{- with .Values.identity.ldap }}
{{- if .enabled }}
{{- if or (not .url) (not .userBaseDN) }}
{{- fail "identity.ldap.enabled needs identity.ldap.url and identity.ldap.userBaseDN" }}
{{- end }}
{{- if and .bindDN (not (include "kubebench.ldapSecretName" $)) }}
{{- fail "identity.ldap.bindDN needs identity.ldap.existingSecret or identity.ldap.bindPassword" }}
{{- end }}
- name: KUBEBENCH_LDAP_URL
  value: {{ .url | quote }}
- name: KUBEBENCH_LDAP_TYPE
  value: {{ .type | quote }}
{{- if .startTLS }}
- name: KUBEBENCH_LDAP_START_TLS
  value: "true"
{{- end }}
{{- if .insecureSkipVerify }}
- name: KUBEBENCH_LDAP_INSECURE_SKIP_VERIFY
  value: "true"
{{- end }}
- name: KUBEBENCH_LDAP_USER_BASE_DN
  value: {{ .userBaseDN | quote }}
{{- range $key, $env := dict "name" "NAME" "bindDN" "BIND_DN" "userFilter" "USER_FILTER" "usernameAttribute" "USERNAME_ATTRIBUTE" "idAttribute" "ID_ATTRIBUTE" "displayNameAttribute" "DISPLAY_NAME_ATTRIBUTE" "emailAttribute" "EMAIL_ATTRIBUTE" "groupsFrom" "GROUPS_FROM" "groupBaseDN" "GROUP_BASE_DN" "groupFilter" "GROUP_FILTER" "groupNameAttribute" "GROUP_NAME_ATTRIBUTE" "timeout" "TIMEOUT" }}
{{- with index $.Values.identity.ldap $key }}
- name: {{ printf "KUBEBENCH_LDAP_%s" $env }}
  value: {{ . | quote }}
{{- end }}
{{- end }}
{{- with include "kubebench.ldapSecretName" $ }}
- name: KUBEBENCH_LDAP_BIND_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ . }}
      key: {{ include "kubebench.ldapSecretKey" $ }}
{{- end }}
{{- if .caBundle }}
- name: KUBEBENCH_LDAP_CA_FILE
  value: {{ printf "%s/ca.crt" (include "kubebench.ldapCADir" $) | quote }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}

{{/* Whether the OIDC CA bundle ConfigMap is mounted. */}}
{{- define "kubebench.oidcCA" -}}
{{- if and .Values.identity.oidc.enabled .Values.identity.oidc.caBundle -}}true{{- end -}}
{{- end -}}
