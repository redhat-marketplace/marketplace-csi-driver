FROM golang:1.15-alpine as builder
ARG ldflags

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY LICENSE .

# Build CSI Driver
RUN CGO_ENABLED=0 GO111MODULE=on go build -ldflags "${ldflags}" -a -o marketplacecsidriver ./cmd/main.go

FROM quay.io/rh-marketplace/marketplace-csi-driver-rhel-base:1.0.0

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

WORKDIR /

RUN echo "user_allow_other" > /etc/fuse.conf

COPY --from=builder /workspace/marketplacecsidriver /usr/local/bin
COPY LICENSE /licenses/

ENTRYPOINT ["/usr/local/bin/marketplacecsidriver"]
