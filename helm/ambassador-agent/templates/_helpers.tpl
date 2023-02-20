{{/*
Expand the name of the chart.
*/}}
{{- define "ambassador-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "ambassador-agent.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "ambassador-agent.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ambassador-agent.labels" -}}
helm.sh/chart: {{ include "ambassador-agent.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "ambassador-agent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ambassador-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Set the image that should be used for ambassador.
Use fullImageOverride if present,
Then if the image repository is explicitly set, use "repository:image"
*/}}
{{- define "ambassador-agent.image" -}}
{{ $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- if .Values.image.fullImageOverride }}
{{- .Values.image.fullImageOverride }}
{{- else if hasKey .Values.image "repository"  -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- else -}}
{{- printf "%s:%s" "docker.io/ambassador/ambassador-agent" $tag -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "ambassador-agent.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ambassador-agent.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{/*
Create chart namespace based on override value.
*/}}
{{- define "ambassador-agent.namespace" -}}
{{- if .Values.namespaceOverride -}}
{{- .Values.namespaceOverride -}}
{{- else -}}
{{- .Release.Namespace -}}
{{- end -}}
{{- end -}}

{{/*
Create the name of the RBAC to use
*/}}
{{- define "ambassador-agent.rbacName" -}}
{{ default (include "ambassador-agent.fullname" .) .Values.rbac.nameOverride }}
{{- end -}}