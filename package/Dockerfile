ARG UBI_IMAGE=registry.access.redhat.com/ubi8/ubi-minimal:latest
FROM ${UBI_IMAGE}
ENV KUBECONFIG /root/.kube/config
RUN microdnf update -y && \
    rm -rf /var/cache/yum
COPY bin/eks-operator /usr/bin/
USER 1001
ENTRYPOINT ["eks-operator"]
