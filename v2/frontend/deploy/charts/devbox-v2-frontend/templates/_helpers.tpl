{{- define "devbox-v2-frontend.image" -}}
{{- printf "%s:%s" .repository .tag -}}
{{- end -}}

{{- define "devbox-v2-frontend.frontendImage" -}}
{{- include "devbox-v2-frontend.image" .Values.frontend.image -}}
{{- end -}}

{{- define "devbox-v2-frontend.initImage" -}}
{{- include "devbox-v2-frontend.image" .Values.frontend.initImage -}}
{{- end -}}

{{- define "devbox-v2-frontend.frontendHost" -}}
{{- printf "%s.%s" (default "devbox" .Values.ingress.hostPrefix) .Values.cloudDomain -}}
{{- end -}}

{{- define "devbox-v2-frontend.scheme" -}}
{{- if eq (toString .Values.disableHttps) "true" -}}http{{- else -}}https{{- end -}}
{{- end -}}

{{- define "devbox-v2-frontend.externalPort" -}}
{{- $scheme := include "devbox-v2-frontend.scheme" . -}}
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

{{- define "devbox-v2-frontend.externalPortSuffix" -}}
{{- $port := include "devbox-v2-frontend.externalPort" . -}}
{{- if $port -}}:{{ $port }}{{- end -}}
{{- end -}}

{{- define "devbox-v2-frontend.rootExternalOrigin" -}}
{{- include "devbox-v2-frontend.scheme" . -}}://{{ .Values.cloudDomain }}{{ include "devbox-v2-frontend.externalPortSuffix" . }}
{{- end -}}

{{- define "devbox-v2-frontend.wildcardExternalOrigin" -}}
{{- include "devbox-v2-frontend.scheme" . -}}://*.{{ .Values.cloudDomain }}{{ include "devbox-v2-frontend.externalPortSuffix" . }}
{{- end -}}

{{- define "devbox-v2-frontend.frontendExternalURL" -}}
{{- include "devbox-v2-frontend.scheme" . -}}://{{ include "devbox-v2-frontend.frontendHost" . }}{{ include "devbox-v2-frontend.externalPortSuffix" . }}
{{- end -}}

{{- define "devbox-v2-frontend.registryConfigValue" -}}
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

{{- define "devbox-v2-frontend.registryAddr" -}}
{{- include "devbox-v2-frontend.registryConfigValue" (list "REGISTRY_ADDR" "sealos.hub:5000" .Values.registry.addr) -}}
{{- end -}}

{{- define "devbox-v2-frontend.registryUser" -}}
{{- include "devbox-v2-frontend.registryConfigValue" (list "ADMIN_USER" "" .Values.registry.user) -}}
{{- end -}}

{{- define "devbox-v2-frontend.registryPassword" -}}
{{- include "devbox-v2-frontend.registryConfigValue" (list "ADMIN_PASSWORD" "" .Values.registry.password) -}}
{{- end -}}

{{- define "devbox-v2-frontend.domainChallengeSecret" -}}
{{- default "" .Values.frontend.env.domainChallengeSecret -}}
{{- end -}}
