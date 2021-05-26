FROM registry.suse.com/suse/sle15:15.3
RUN zypper -n update && \
    zypper -n clean -a && \
    rm -rf /tmp/* /var/tmp/* /usr/share/doc/packages/*
RUN useradd --uid 1007 eks-operator
ENV KUBECONFIG /home/eks-operator/.kube/config
ENV SSL_CERT_DIR /etc/rancher/ssl

COPY bin/eks-operator /usr/bin/
COPY package/entrypoint.sh /usr/bin
RUN chmod +x /usr/bin/entrypoint.sh

RUN mkdir -p /etc/rancher/ssl && \
    chown -R eks-operator /etc/rancher/ssl

USER 1007
ENTRYPOINT ["entrypoint.sh"]
