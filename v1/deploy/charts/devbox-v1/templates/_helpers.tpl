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

{{- define "devbox-v1.registryConfigValue" -}}
{{- $key := index . 0 -}}
{{- $fallback := index . 1 -}}
{{- $value := index . 2 -}}
{{- if not $value -}}
{{- with (lookup "v1" "ConfigMap" "sealos-system" "registry-config") -}}
{{- $value = index .data $key | default "" -}}
{{- end -}}
{{- end -}}
{{- default $fallback $value -}}
{{- end -}}

{{- define "devbox-v1.registryAddr" -}}
{{- include "devbox-v1.registryConfigValue" (list "REGISTRY_ADDR" "sealos.hub:5000" .Values.registry.addr) -}}
{{- end -}}

{{- define "devbox-v1.registryUser" -}}
{{- include "devbox-v1.registryConfigValue" (list "ADMIN_USER" "admin" .Values.registry.user) -}}
{{- end -}}

{{- define "devbox-v1.registryPassword" -}}
{{- include "devbox-v1.registryConfigValue" (list "ADMIN_PASSWORD" "passw0rd" .Values.registry.password) -}}
{{- end -}}
