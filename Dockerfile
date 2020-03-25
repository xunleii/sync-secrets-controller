ARG ARCH
ARG OS

FROM quay.io/prometheus/busybox-${OS}-${ARCH}:glibc
LABEL maintainer="Alexandre NICOLAIE <alexandre.nicolaie@gmail.com>"

LABEL org.opencontainers.image.authors="Alexandre NICOLAIE <alexandre.nicolaie@gmail.com>"
LABEL org.opencontainers.image.description="Operator syncing secrets across namespaces in Kubernetes"
LABEL org.opencontainers.image.licenses="Apache-2.0"
LABEL org.opencontainers.image.source="https://github.com/xunleii/sync-secrets-controller"

COPY controller /bin/controller
COPY LICENSE /LICENSE

USER       nobody
EXPOSE     8080
WORKDIR    /
ENTRYPOINT [ "/bin/controller" ]
CMD        [ "-v1", "--ignore-namespaces=kube-system" ]