# Image that provides cross compilation tooling.
FROM --platform=$BUILDPLATFORM rancher/mirrored-tonistiigi-xx:1.5.0 AS xx

FROM registry.suse.com/bci/bci-base:15.6 AS base
RUN sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd
RUN useradd --uid 1007 eks-operator

FROM --platform=$BUILDPLATFORM registry.suse.com/bci/golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY ./controller ./controller
COPY ./pkg ./pkg
COPY ./templates ./templates
COPY ./utils ./utils
COPY ./main.go ./main.go

# Copy xx scripts to your build stage
COPY --from=xx / /

ARG TARGETPLATFORM
ARG COMMIT
ARG VERSION
ENV CGO_ENABLED=0#
RUN xx-go build -ldflags \
           "-X github.com/rancher/eks-operator/pkg/version.GitCommit=${COMMIT} \
            -X github.com/rancher/eks-operator/pkg/version.Version=${VERSION}" \
       -o /eks-operator && \
       xx-verify /eks-operator

FROM registry.suse.com/bci/bci-micro:15.6
COPY --from=base /etc/passwd /etc/passwd
COPY --from=base /etc/shadow /etc/shadow
COPY --from=builder /eks-operator /usr/bin/eks-operator

RUN rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/*

ENV KUBECONFIG="/home/eks-operator/.kube/config"
ENV SSL_CERT_DIR="/etc/rancher/ssl"

COPY package/entrypoint.sh /usr/bin
RUN chmod +x /usr/bin/entrypoint.sh

RUN mkdir -p /etc/rancher/ssl && \
    chown -R eks-operator /etc/rancher/ssl

USER 1007
ENTRYPOINT ["entrypoint.sh"]
