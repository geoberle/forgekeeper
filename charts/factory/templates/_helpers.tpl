{{- define "factory.name" -}}
{{- printf "%s-factory" .Release.Name | trunc 50 | trimSuffix "-" -}}
{{- end -}}
{{- define "factory.image" -}}
{{- printf "%s:%s" .Values.image.repository (required "Set image.tag to a built immutable image tag" .Values.image.tag) -}}
{{- end -}}
{{- define "factory.security" -}}
runAsNonRoot: true
runAsUser: 1000
runAsGroup: 1000
fsGroup: 1000
fsGroupChangePolicy: OnRootMismatch
seccompProfile:
  type: RuntimeDefault
{{- end -}}
{{- define "factory.containerSecurity" -}}
allowPrivilegeEscalation: false
capabilities:
  drop: [ALL]
{{- end -}}
