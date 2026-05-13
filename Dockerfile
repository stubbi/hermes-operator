ARG PREBUILT_BINARY=""

# Build the manager binary
FROM golang:1.26 AS builder
ARG PREBUILT_BINARY
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN if [ -z "$PREBUILT_BINARY" ]; then go mod download; fi

# Copy the go source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/

# GoReleaser will COPY a prebuilt binary in; otherwise build from source.
COPY ${PREBUILT_BINARY:-cmd/main.go} ./prebuilt-or-main
RUN set -eu; \
    if [ -n "${PREBUILT_BINARY:-}" ]; then \
      cp ./prebuilt-or-main /workspace/manager && chmod +x /workspace/manager; \
    else \
      rm ./prebuilt-or-main && \
      CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o /workspace/manager cmd/main.go; \
    fi

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
