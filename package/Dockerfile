FROM registry.suse.com/suse/sle15:15.3
RUN zypper -n update && \
    zypper -n clean -a && \
    rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/*
RUN useradd --uid 1007 eks-operator
ENV KUBECONFIG /home/eks-operator/.kube/config
COPY bin/eks-operator /usr/bin/
USER 1007
ENTRYPOINT ["eks-operator"]
