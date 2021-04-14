# Current Operator version
VERSION ?= 1.0.3
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

TAG ?= $(VERSION)
IMAGE_REGISTRY ?= quay.io/rh-marketplace

OPERATOR_IMAGE_NAME = marketplace-dataset-operator
OPERATOR_IMAGE_TAG ?= $(TAG)
OPERATOR_IMAGE ?= $(IMAGE_REGISTRY)/$(OPERATOR_IMAGE_NAME):$(OPERATOR_IMAGE_TAG)

BUNDLE_IMAGE_VERSION ?= 1.0.7
BUNDLE_IMAGE_NAME = marketplace-dataset-operator-manifest
BUNDLE_IMAGE ?= $(IMAGE_REGISTRY)/$(BUNDLE_IMAGE_NAME):$(BUNDLE_IMAGE_VERSION)

DRIVER_IMAGE_VERSION ?= 1.0.2
DRIVER_IMAGE_NAME = marketplace-csi-driver
DRIVER_IMAGE ?= $(IMAGE_REGISTRY)/$(DRIVER_IMAGE_NAME)
LDFLAGS := "-X main.driverVersion=$(DRIVER_IMAGE_VERSION) -X main.driverImage=$(DRIVER_IMAGE)"

QUAY_EXPIRATION?=never

# Image URL to use all building/pushing image targets
IMG ?= $(OPERATOR_IMAGE)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

PLATFORMS ?= linux/amd64,linux/ppc64le,linux/s390x

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
test: generate bindata fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh	
	. $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test -v ./controllers ./pkg/... -coverpkg=$(shell go list ./... | egrep -v '(assets|api|status)' | tr '\n' ',') -coverprofile cover.out

teste2e:
	SKIP_DEPLOY=true kubectl kuttl test --namespace openshift-redhat-marketplace --kind-context test --config ./kuttl-test.yaml ../tests/e2e --suppress-log "events"

testci: generate bindata fmt vet manifests
	go test -v ./controllers ./pkg/... -coverpkg=$(shell go list ./... | egrep -v '(assets|api|status)' | tr '\n' ',') -coverprofile cover.out

# Build manager binary
manager: generate bindata fmt vet
	go build -o bin/manager -ldflags $(LDFLAGS) main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default-k8s | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

docker-publish:
	docker buildx build --tag ${IMG} \
		--build-arg name="Dataset Operator" \
		--build-arg app_version="$(VERSION)" \
		--build-arg quay_expiration="$(QUAY_EXPIRATION)" \
		--build-arg ldflags=$(LDFLAGS) \
		--platform $(PLATFORMS) --push .  \

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

undeployolm:
	$(KUSTOMIZE) build config/default | kubectl delete -f -

undeploy:
	$(KUSTOMIZE) build config/default-k8s | kubectl delete -f -

devmanifest:
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default-k8s > deploy/dev-operator.yaml

bindata:
	go-bindata -pkg assets -o pkg/assets/bindata.go assets/...

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Publish the bundle image.
.PHONY: bundle-push
bundle-push:
	docker build -f bundle.Dockerfile \
		--label com.redhat.openshift.versions="v4.5,v4.6,v4.7" \
		--label com.redhat.delivery.operator.bundle=true \
		--label com.redhat.delivery.backport=true \
		--tag $(BUNDLE_IMAGE) --push .
	
bundle-push-dev:
	docker build -f bundle.Dockerfile \
		--label com.redhat.openshift.versions="v4.5,v4.6,v4.7" \
		--label com.redhat.delivery.operator.bundle=true \
		--label com.redhat.delivery.backport=true \
		--tag $(BUNDLE_IMAGE) .
	docker push $(BUNDLE_IMAGE)