{{- define "devbox-v1.image" -}}
{{- printf "%s:%s" .repository .tag -}}
{{- end -}}

{{- define "devbox-v1.frontendHost" -}}
{{- printf "%s.%s" (default "devbox" .Values.ingress.hostPrefix) .Values.cloudDomain -}}
{{- end -}}

{{- define "devbox-v1.scheme" -}}
{{- $disableHttps := .Values.disableHttps -}}
{{- if eq (toString $disableHttps) "true" -}}http{{- else -}}https{{- end -}}
{{- end -}}

{{- define "devbox-v1.externalPort" -}}
{{- $scheme := include "devbox-v1.scheme" . -}}
{{- $port := toString .Values.cloudPort -}}
{{- if eq $scheme "http" -}}
{{- $port = toString .Values.httpPort -}}
{{- end -}}
{{- if or (and (eq $scheme "https") (or (eq $port "") (eq $port "443"))) (and (eq $scheme "http") (or (eq $port "") (eq $port "80"))) -}}
{{- "" -}}
{{- else -}}
{{- $port -}}
{{- end -}}
{{- end -}}

{{- define "devbox-v1.externalPortSuffix" -}}
{{- $port := include "devbox-v1.externalPort" . -}}
{{- if $port -}}:{{ $port }}{{- end -}}
{{- end -}}

{{- define "devbox-v1.rootExternalOrigin" -}}
{{- include "devbox-v1.scheme" . -}}://{{ .Values.cloudDomain }}{{ include "devbox-v1.externalPortSuffix" . }}
{{- end -}}

{{- define "devbox-v1.wildcardExternalOrigin" -}}
{{- include "devbox-v1.scheme" . -}}://*.{{ .Values.cloudDomain }}{{ include "devbox-v1.externalPortSuffix" . }}
{{- end -}}

{{- define "devbox-v1.frontendExternalURL" -}}
{{- include "devbox-v1.scheme" . -}}://{{ include "devbox-v1.frontendHost" . }}{{ include "devbox-v1.externalPortSuffix" . }}
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
