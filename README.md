# Secret synchronisation controller

[![GoReleaser Workflow](https://github.com/xunleii/sync-secrets-controller/workflows/GoReleaser/badge.svg)](https://github.com/xunleii/sync-secrets-controller/actions)
[![Go test & linter Workflow](https://github.com/xunleii/sync-secrets-controller/workflows/Go%20-%20Test%20&%20Lint/badge.svg)](https://github.com/xunleii/sync-secrets-controller/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/xunleii/sync-secrets-controller)](https://goreportcard.com/report/github.com/xunleii/sync-secrets-controller)
[![Go Version Card](https://img.shields.io/github/go-mod/go-version/xunleii/sync-secrets-controller)](go.mod)

## Overview

> Sometimes, we need to access to a secret from an another namespace, which is impossible because secret are namespaced
> and only accessible to the secret's namespace.
> For example, if we have a CA certificate in the namespace A and we want to use it in the namespace B, in order to
> create a new certificate, we need to create a new secret in B with the content of A. Moreover, because the original
> secret can be updated, we always need to sync it to the namespace B, manually.

That is why this controller exists. Thanks to annotations on a secret, it can automatically synchronise the secret over
several namespaces.  *However, I do not recommend to use this controller for anything ;
[Kubernetes Secret's restrictions](https://kubernetes.io/docs/concepts/configuration/secret/#restrictions) are here
for a good reason and this controller breaks one of theses restrictions.*

## Annotations

> **These annotations cannot be used together**

`secret.sync.klst.pw/all-namespaces: 'true'`: Synchronize the current secret over all namespace
`secret.sync.klst.pw/namespace-selector: LABEL_SELECTOR`: Synchronize the current secret over all namespace
validating the given label selector

## Features

**This controller can:**

- Synchronize a secret over all namespaces
- Synchronize a secret on specifics namespaces, thanks label selectors
- Synchronize on a new namespace when a secret is already "synchronized"
- Automatically update "slave" secrets when the original is update
- Automatically restore "slave" secret when it is manually modified
- Automatically remove "slave" secrets when the original is removed
- Automatically remove/update "slave" secrets when the original secret annotations are modified/removed
- Automatically recreate "slave" secret when it is removed

## Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  annotations:
    secret.sync.klst.pw/namespace-selector: require-creds=admin
  name: admin-creds
  namespace: default
type: Opaque
data:
  username: YWRtaW4=
  password: MWYyZDFlMmU2N2Rm
```

This secret will be synchronized on all namespaces with the label `require-creds: admin`. For more information about
label selector, see [Kubernetes label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors.)

## How to install

You can install the deployment in a kubernetes cluster with the following commands

```bash
kubectl apply -f https://github.com/xunleii/sync-secrets-controller/tree/master/deploy/rbac.yaml
kubectl apply -f https://github.com/xunleii/sync-secrets-controller/tree/master/deploy/deployment.yaml
```

---

*This controller is still under development and may introduce breaking changes between versions.
Please check the [CHANGELOG](CHANGELOG.md) before updating.*

[Apache License 2.0](LICENSE)
