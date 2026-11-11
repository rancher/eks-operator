# Minimal BCI base image, as recommended by RFD 0070.
FROM registry.suse.com/bci/bci-nano:16.1 AS base

# bci-nano has no shell to create the eks-operator user or the directories it
# needs, so the files are prepared here and copied into the final image.
FROM --platform=$BUILDPLATFORM registry.suse.com/bci/bci-base:16.1 AS user-setup
COPY --from=base /etc/passwd /final/etc/passwd
COPY --from=base /etc/group /final/etc/group
RUN echo "eks-operator:x:1007:1007::/home/eks-operator:/usr/bin/false" >> /final/etc/passwd && \
    echo "eks-operator:x:1007:" >> /final/etc/group && \
    install -dv -m 0755 -o 1007 -g 1007 /final/etc/rancher/ssl

# Builds with the Go compiler from the AppCo Go image, as recommended by
# RFD 0070. Pinned to MAJOR.MINOR.PATCH so the toolchain is deterministic and
# bumped by Renovate. It has no package manager, and does not need one: Go
# cross compiles with GOOS/GOARCH, as rancher/rancher does.
FROM --platform=$BUILDPLATFORM registry.suse.com/rancher/appco-go:1.27.3 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

COPY ./controller ./controller
COPY ./pkg ./pkg
COPY ./templates ./templates
COPY ./utils ./utils
COPY ./main.go ./main.go

ARG TARGETOS
ARG TARGETARCH
ARG COMMIT
ARG VERSION
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags \
           "-X github.com/rancher/eks-operator/pkg/version.GitCommit=${COMMIT} \
            -X github.com/rancher/eks-operator/pkg/version.Version=${VERSION}" \
       -o /out/eks-operator

FROM base AS final

COPY --from=user-setup /final/ /
COPY --from=builder /out/eks-operator /usr/bin/eks-operator

ENV KUBECONFIG="/home/eks-operator/.kube/config"
# eks-operator is a static Go binary, which loads every certificate file in
# SSL_CERT_DIR, so no c_rehash step is needed for additional CAs.
ENV SSL_CERT_DIR="/etc/rancher/ssl"

USER 1007
ENTRYPOINT ["/usr/bin/eks-operator"]
