ARG ARCH

FROM ${ARCH}/busybox:glibc
ARG ARCH

LABEL maintainer="Alexandre NICOLAIE <alexandre.nicolaie@gmail.com>"
LABEL org.opencontainers.image.authors="Alexandre NICOLAIE <alexandre.nicolaie@gmail.com>"
LABEL org.opencontainers.image.description="Controller syncing secrets across namespaces in Kubernetes"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/xunleii/sync-secrets-controller"

COPY controller-${ARCH} /bin/controller
COPY LICENSE /LICENSE

USER       nobody
EXPOSE     8080
WORKDIR    /
ENTRYPOINT [ "/bin/controller" ]
CMD        [ "-v1", "--ignore-namespaces=kube-system" ]