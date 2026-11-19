{{- define "wa.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "wa.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "wa.selectorLabels" -}}
app.kubernetes.io/name: {{ include "wa.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "wa.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "wa.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "wa.image" -}}
{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}
{{- end }}

{{/* The name of the ConfigMap holding the issuer, audience, client IDs and the UI's config.json. */}}
{{- define "wa.oidcConfigMap" -}}
{{ include "wa.fullname" . }}-oidc
{{- end }}

{{- define "wa.collectorName" -}}
{{ include "wa.fullname" . }}-collector
{{- end }}

{{- define "wa.uiName" -}}
{{ include "wa.fullname" . }}-ui
{{- end }}

{{/* The managed CloudNativePG Cluster's name. */}}
{{- define "wa.dbClusterName" -}}
{{ include "wa.fullname" . }}-db
{{- end }}

{{/* The Secret holding the three generated URLs in managed mode. */}}
{{- define "wa.dbUrlsSecret" -}}
{{ include "wa.fullname" . }}-db-urls
{{- end }}

{{/*
The UI's runtime config.json (docs/architecture/ui-contract.md), for the web
app's client ID.
Usage: include "wa.uiConfig" (list . $clientId)
*/}}
{{- define "wa.uiConfig" -}}
{{- $root := index . 0 }}
{{- $oidc := $root.Values.oidc }}
{{- dict "apiBaseUrl" "/api" "oidc" (dict
  "issuer" $oidc.issuer
  "clientId" (index . 1)
  "audience" $oidc.audience
  "redirectPath" $oidc.redirectPath
  "postLogoutRedirectPath" $oidc.postLogoutRedirectPath) | toJson }}
{{- end }}

{{/*
The scheme, host and port the UI is served from. The port is left out when
it is unset or the scheme's default.
*/}}
{{- define "wa.origin" -}}
{{- $scheme := ternary "https" "http" .Values.route.tls }}
{{- $port := int (.Values.route.port | default 0) }}
{{- $scheme }}://{{ .Values.route.hostname }}
{{- if and $port (ne $port (ternary 443 80 .Values.route.tls)) }}:{{ $port }}{{ end }}
{{- end }}

{{/*
A secretKeyRef for one of the database URLs: owner, app or reader.
Usage: include "wa.dbUrlRef" (list . "owner")
*/}}
{{- define "wa.dbUrlRef" -}}
{{- $root := index . 0 }}
{{- $which := index . 1 }}
{{- if eq $root.Values.database.mode "managed" }}
secretKeyRef:
  name: {{ include "wa.dbUrlsSecret" $root }}
  key: {{ $which }}-url
{{- else }}
{{- $ref := index $root.Values.database.external (printf "%sUrl" $which) }}
secretKeyRef:
  name: {{ $ref.secretName }}
  key: {{ $ref.key }}
{{- end }}
{{- end }}

{{/*
The environment every app container gets, except the owner URL: the app and
reader URLs, OIDC, OpenTelemetry and extra CAs.
*/}}
{{- define "wa.appEnv" -}}
- name: WA_DATABASE_URL
  valueFrom:
    {{- include "wa.dbUrlRef" (list . "app") | nindent 4 }}
- name: WA_READER_DATABASE_URL
  valueFrom:
    {{- include "wa.dbUrlRef" (list . "reader") | nindent 4 }}
- name: WA_OIDC_ISSUER
  value: {{ .Values.oidc.issuer | quote }}
- name: WA_OIDC_AUDIENCE
  value: {{ .Values.oidc.audience | quote }}
- name: WA_HTTP_ADDR
  value: ":8080"
{{- if or .Values.collector.enabled .Values.collector.externalEndpoint }}
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: {{ ternary (printf "http://%s:4318" (include "wa.collectorName" .)) .Values.collector.externalEndpoint .Values.collector.enabled | quote }}
- name: OTEL_EXPORTER_OTLP_PROTOCOL
  value: http/protobuf
{{- else }}
{{- range $signal := list "TRACES" "METRICS" "LOGS" }}
- name: OTEL_{{ $signal }}_EXPORTER
  value: none
{{- end }}
{{- end }}
- name: OTEL_SERVICE_NAME
  value: wallet-accountant
{{- if .Values.trust.caConfigMap.name }}
- name: SSL_CERT_DIR
  value: /etc/ssl/extra-ca
{{- end }}
{{- with .Values.app.extraEnv }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/* Volume mounts and volumes for the extra CAs. */}}
{{- define "wa.caVolumeMounts" -}}
{{- if .Values.trust.caConfigMap.name }}
- name: extra-ca
  mountPath: /etc/ssl/extra-ca
  readOnly: true
{{- end }}
{{- end }}

{{- define "wa.caVolumes" -}}
{{- if .Values.trust.caConfigMap.name }}
- name: extra-ca
  configMap:
    name: {{ .Values.trust.caConfigMap.name }}
    items:
      - key: {{ .Values.trust.caConfigMap.key }}
        path: ca.crt
{{- end }}
{{- end }}

{{/*
The pod security context, for a workload whose image runs as uid.
Usage: include "wa.podSecurityContext" (list . 65532)
*/}}
{{- define "wa.podSecurityContext" -}}
{{- $root := index . 0 }}
{{- $uid := index . 1 }}
{{- toYaml (merge (deepCopy $root.Values.podSecurityContext) (dict "runAsUser" $uid "runAsGroup" $uid "fsGroup" $uid)) }}
{{- end }}
