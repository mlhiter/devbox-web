{{- define "devbox-v1.image" -}}
{{- printf "%s:%s" .repository .tag -}}
{{- end -}}

{{- define "devbox-v1.frontendHost" -}}
{{- printf "%s.%s" (default "devbox" .Values.ingress.hostPrefix) .Values.cloudDomain -}}
{{- end -}}

{{- define "devbox-v1.frontendExternalURL" -}}
{{- printf "https://%s" (include "devbox-v1.frontendHost" .) -}}{{- if .Values.cloudPort -}}:{{ .Values.cloudPort }}{{- end -}}
{{- end -}}

{{- define "devbox-v1.frontendImage" -}}
{{- include "devbox-v1.image" .Values.frontend.image -}}
{{- end -}}

{{- define "devbox-v1.controllerImage" -}}
{{- include "devbox-v1.image" .Values.controller.image -}}
{{- end -}}
