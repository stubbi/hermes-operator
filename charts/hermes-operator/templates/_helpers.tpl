{{- define "hermes-operator.fullname" -}}
{{- printf "%s" (default .Chart.Name .Values.nameOverride) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "hermes-operator.labels" -}}
app.kubernetes.io/name: {{ include "hermes-operator.fullname" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: hermes.agent
{{- end -}}

{{- define "hermes-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hermes-operator.fullname" . }}
{{- end -}}

{{- define "hermes-operator.image" -}}
{{- $tag := default (printf "v%s" .Chart.AppVersion) .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}
