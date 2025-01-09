FROM registry.suse.com/bci/bci-base:15.6 AS builder
RUN sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd
RUN useradd --uid 1007 eks-operator

FROM registry.suse.com/bci/bci-micro:15.6
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/shadow /etc/shadow

RUN rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/*

ENV KUBECONFIG="/home/eks-operator/.kube/config"
ENV SSL_CERT_DIR="/etc/rancher/ssl"

# Once this image is migrated to be SLSA compliant and the Go build happens
# inside a build layer, we must pass the version and commit ID to the build,
# similar to what was done in https://github.com/rancher/aks-operator/pull/803 .
# This is just a reference for future changes, because it's needed for our VEX
# work.
COPY bin/eks-operator /usr/bin/
COPY package/entrypoint.sh /usr/bin
RUN chmod +x /usr/bin/entrypoint.sh

RUN mkdir -p /etc/rancher/ssl && \
    chown -R eks-operator /etc/rancher/ssl

USER 1007
ENTRYPOINT ["entrypoint.sh"]
