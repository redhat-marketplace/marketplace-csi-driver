VERSION ?= 1.0.0

PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))

IMAGE_REGISTRY ?= quay.io/grishgo

DRIVER_IMAGE_NAME = marketplace-csi-driver
DRIVER_IMAGE_TAG ?= $(VERSION)
DRIVER_IMAGE ?= $(IMAGE_REGISTRY)/$(DRIVER_IMAGE_NAME):$(DRIVER_IMAGE_TAG)

BASE_IMAGE_VERSION ?= 0.0.3
BASE_IMAGE_NAME = marketplace-csi-driver-rhel-base
BASE_IMAGE ?= $(IMAGE_REGISTRY)/$(BASE_IMAGE_NAME):$(BASE_IMAGE_VERSION)

QUAY_EXPIRATION?=never

# Keep an existing GOPATH, make a private one if it is undefined
GOPATH_DEFAULT := $(PWD)/.go
export GOPATH ?= $(GOPATH_DEFAULT)
TESTARGS_DEFAULT := "-v"
export TESTARGS ?= $(TESTARGS_DEFAULT)
DEST := $(GOPATH)/src/$(GIT_HOST)/$(BASE_DIR)

LDFLAGS := "-X main.version=$(DRIVER_IMAGE_TAG)"
PLATFORMS ?= linux/amd64

LOCAL_OS := $(shell uname)
ifeq ($(LOCAL_OS),Linux)
    TARGET_OS ?= linux
    XARGS_FLAGS="-r"
else ifeq ($(LOCAL_OS),Darwin)
    TARGET_OS ?= darwin
    XARGS_FLAGS=
else
    $(error "This system's OS $(LOCAL_OS) isn't recognized/supported")
endif

############################################################
# build section
############################################################

build: fmt
	@echo "Building the $(DRIVER_IMAGE_NAME) binary..."
	@CGO_ENABLED=0 go build -ldflags $(LDFLAGS) -o build/_output/bin/$(DRIVER_IMAGE_NAME) ./cmd/

############################################################
# image section
############################################################

image: build-image push-image

build-image: build
	@echo "Building the docker image: $(DRIVER_IMAGE)..."
	@docker build -t $(DRIVER_IMAGE) --build-arg ldflags=$(LDFLAGS) -f Dockerfile .

push-image: build-image
	@echo "Pushing the docker image for $(DRIVER_IMAGE)"
	@docker push $(DRIVER_IMAGE)

docker-publish:
	docker build --tag ${DRIVER_IMAGE} \
		--build-arg name="CSI Driver" \
		--build-arg ldflags=$(LDFLAGS) \
		--build-arg app_version="$(VERSION)" \
		--build-arg quay_expiration="$(QUAY_EXPIRATION)" --push .

docker-publish-full:
	docker build --tag ${DRIVER_IMAGE} \
		--build-arg name="CSI Driver" \
		--build-arg ldflags=$(LDFLAGS) \
		--build-arg app_version="$(VERSION)" \
		--build-arg quay_expiration="$(QUAY_EXPIRATION)" \
		--push -f Dockerfile.full . 

docker-publish-base:
	docker build --no-cache --tag ${BASE_IMAGE} --push -f Dockerfile.base .

fmt:
	go fmt ./...

test: build
	go test -v ./pkg/... -coverpkg=$(shell go list ./... | tr '\n' ',') -coverprofile cover.out

############################################################
# clean section
############################################################
clean:
	@rm -rf build/_output

.PHONY: build clean