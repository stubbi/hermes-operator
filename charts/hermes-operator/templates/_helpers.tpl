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
{{- /*
Fall back to the chart's appVersion, normalised to the v-prefixed form the
release workflow publishes. appVersion is written bare by release-please in
Chart.yaml but a packaged chart may carry it v-prefixed, so strip any existing
"v" before adding one - otherwise the tag renders as "vv0.1.19".
*/ -}}
{{- $tag := default (printf "v%s" (trimPrefix "v" .Chart.AppVersion)) .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}

{{- /*
Manager RBAC, split by scope so watchNamespaces can narrow the grant (#126).

clusterScopedRules covers cluster-scoped resources only and is always bound
cluster-wide. namespacedRules covers everything the manager touches inside a
reconciled namespace: it goes into the ClusterRole when watchNamespaces is
empty, and into one Role per watched namespace otherwise. Together they must
equal config/rbac/role.yaml; hack/check-helm-rbac.sh enforces that.
*/ -}}
{{- define "hermes-operator.clusterScopedRules" -}}
- apiGroups: [hermes.agent]
  resources: [hermesclusterdefaults]
  verbs: [get, list, patch, update, watch]
- apiGroups: [hermes.agent]
  resources: [hermesclusterdefaults/status]
  verbs: [get, patch, update]
{{- end -}}

{{- define "hermes-operator.namespacedRules" -}}
- apiGroups: [""]
  resources: [configmaps, events, persistentvolumeclaims, secrets, serviceaccounts, services]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [""]
  resources: [pods]
  verbs: [get, list, watch]
- apiGroups: [apps]
  resources: [deployments, statefulsets]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [autoscaling]
  resources: [horizontalpodautoscalers]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [batch]
  resources: [cronjobs, jobs]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [gateway.networking.k8s.io]
  resources: [httproutes]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [hermes.agent]
  resources: [hermesinstances/status, hermesselfconfigs/status]
  verbs: [get, patch, update]
- apiGroups: [hermes.agent]
  resources: [hermesinstances, hermesselfconfigs]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [hermes.agent]
  resources: [hermesinstances/finalizers, hermesselfconfigs/finalizers]
  verbs: [update]
- apiGroups: [monitoring.coreos.com]
  resources: [prometheusrules, servicemonitors]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [networking.k8s.io]
  resources: [ingresses, networkpolicies]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [openclaw.rocks]
  resources: [openclawinstances]
  verbs: [get, list, watch]
- apiGroups: [policy]
  resources: [poddisruptionbudgets]
  verbs: [create, delete, get, list, patch, update, watch]
- apiGroups: [rbac.authorization.k8s.io]
  resources: [rolebindings, roles]
  verbs: [create, delete, get, list, patch, update, watch]
{{- end -}}

{{- /* namespaceSelector limiting namespaced admission webhooks to watchNamespaces. */ -}}
{{- define "hermes-operator.webhookNamespaceSelector" -}}
{{- with .Values.watchNamespaces -}}
namespaceSelector:
  matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: {{- toYaml . | nindent 8 }}
{{- end }}
{{- end -}}
