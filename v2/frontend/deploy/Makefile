SEALOS ?= sealos
DOCKER ?= docker
ARCH ?= amd64
IMAGE ?= devbox-v2-frontend-cluster:latest

.PHONY: all
all: lint

.PHONY: lint
lint:
	helm lint charts/devbox-v2-frontend
	helm template devbox-v2-frontend charts/devbox-v2-frontend --namespace devbox-frontend >/tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'name: "devbox-frontend-runtime"' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'name: ENABLE_ADVANCED_CONFIG' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'value: "true"' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'name: METRICS_URL' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'vmselect-vm-stack-victoria-metrics-k8s-stack.vm.svc.cluster.local' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'name: STORAGE_LIMIT' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'value: "20Gi"' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'secretRef:' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'name: APP_LAUNCHPAD_URL' /tmp/devbox-v2-frontend-rendered.yaml
	@grep -q -- 'name: GPU_SCHEDULER_MODE' /tmp/devbox-v2-frontend-rendered.yaml
	helm template devbox-v2-frontend charts/devbox-v2-frontend --namespace devbox-frontend --set-string cloudDomain=example.test --set-string cloudPort=80 --set-string disableHttps=true >/tmp/devbox-v2-frontend-http-rendered.yaml
	@! grep -q -- 'secretName: "wildcard-cert"' /tmp/devbox-v2-frontend-http-rendered.yaml || (echo "HTTP mode must not render TLS secret" >&2; exit 1)

.PHONY: registry-save
registry-save:
	$(SEALOS) registry save --registry-dir=registry_$(ARCH) --arch $(ARCH) .

.PHONY: docker-build
docker-build:
	$(DOCKER) buildx build --platform linux/$(ARCH) -f Kubefile -t $(IMAGE)-$(ARCH) .
