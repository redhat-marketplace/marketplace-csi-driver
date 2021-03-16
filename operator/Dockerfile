# Build the manager binary
FROM --platform=${BUILDPLATFORM} golang:1.15-alpine as builder
ARG TARGETOS
ARG TARGETARCH
ARG ldflags

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY LICENSE .

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -a -ldflags "${ldflags}" -o dsoperator main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ARG name
ARG app_version
ARG quay_expiration

LABEL name="Red Hat Marketplace ${name}" \
  maintainer="rhmoper@us.ibm.com" \
  vendor="Red Hat Marketplace" \
  release="1" \
  summary="Red Hat Marketplace ${name} Image" \
  description="Container for the Red Hat Marketplace ${name}" \
  version="${app_version}" \
  quay.expires-after=${quay_expiration}

RUN microdnf update --setopt=tsflags=nodocs -y \
  && microdnf install procps-ng \
  && microdnf clean all \
  && rm -rf /var/cache/yum

ENV USER_UID=1001 \
  USER_NAME=redhat-marketplace-operator \
  BINFILE=dsoperator

WORKDIR /

COPY --from=builder /workspace/dsoperator /usr/local/bin
COPY hack/docker/entrypoint ./manager
COPY hack/docker/user_setup /usr/local/bin/user_setup
COPY LICENSE /licenses/

RUN  /usr/local/bin/user_setup
USER ${USER_UID}


ENTRYPOINT ["/manager"]