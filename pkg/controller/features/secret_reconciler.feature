@secret
Feature: Secret synchronization on secret's updates
  Secrets should always be synchronized when
  the original Secret is created, updated or removed.

  Background:
    Given Kubernetes must have the following resources
      | ApiGroupVersion | Kind      | Namespace | Name        |
      | v1              | Namespace |           | kube-system |
      | v1              | Namespace |           | kube-public |
      | v1              | Namespace |           | default     |
    And Kubernetes labelizes v1/Namespace 'kube-public' with 'sync=secret'

  @not_exists
  Scenario: Secret doesn't exists
    When the secret reconciler reconciles 'default/secret-not-exists'
    Then nothing occurs
    But Kubernetes doesn't have v1/Secret 'kube-public/secret'
    And Kubernetes doesn't have v1/Secret 'kube-system/secret'

  @no_update
  Scenario: Secret doesn't have annotation
    Given Kubernetes must have v1/Secret 'default/secret'
    When the secret reconciler reconciles 'default/secret'
    Then nothing occurs
    But Kubernetes doesn't have v1/Secret 'kube-public/secret'
    And Kubernetes doesn't have v1/Secret 'kube-system/secret'

  @create
  Scenario: Secret is created with both annotations
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/all-namespaces: 'true'
        secret.sync.klst.pw/namespace-selector: sync=secret
    """
    When the secret reconciler reconciles 'default/secret'
    Then Kubernetes has v1/Secret 'default/secret'
    But Kubernetes doesn't have v1/Secret 'kube-public/secret'
    And Kubernetes doesn't have v1/Secret 'kube-system/secret'

  @create
  Scenario: Secret is created with 'secret.sync.klst.pw/all-namespaces'
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/all-namespaces: 'true'
    data:
      username: bXktYXBw
      password: Mzk1MjgkdmRnN0pi
    """
    When the secret reconciler reconciles 'default/secret'
    Then Kubernetes resource v1/Secret 'kube-public/secret' is similar to 'default/secret'
    And Kubernetes resource v1/Secret 'kube-public/secret' has label 'secret.sync.klst.pw/origin.name=secret'
    And Kubernetes resource v1/Secret 'kube-public/secret' has label 'secret.sync.klst.pw/origin.namespace=default'
    And Kubernetes resource v1/Secret 'kube-system/secret' is equal to 'kube-public/secret'

  @create
  Scenario: Secret is created with 'secret.sync.klst.pw/namespace-selector'
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/namespace-selector: sync=secret
    data:
      username: bXktYXBw
      password: Mzk1MjgkdmRnN0pi
    """
    When the secret reconciler reconciles 'default/secret'
    Then Kubernetes resource v1/Secret 'kube-public/secret' is similar to 'default/secret'
    And Kubernetes resource v1/Secret 'kube-public/secret' has label 'secret.sync.klst.pw/origin.name=secret'
    And Kubernetes resource v1/Secret 'kube-public/secret' has label 'secret.sync.klst.pw/origin.namespace=default'
    But Kubernetes doesn't have v1/Secret 'kube-system/secret'

  @create
  Scenario: Secret is created on an ignored namespace
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/all-namespaces: 'true'
    """
    And the v1/Namespace 'default' is ignored by the reconciler
    When the secret reconciler reconciles 'default/secret'
    Then Kubernetes has v1/Secret 'default/secret'
    But Kubernetes doesn't have v1/Secret 'kube-public/secret'
    And Kubernetes doesn't have v1/Secret 'kube-system/secret'

  @create
  Scenario: Namespace where a secret will be synced is ignored
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/all-namespaces: 'true'
    """
    And the v1/Namespace 'kube-public' is ignored by the reconciler
    When the secret reconciler reconciles 'default/secret'
    Then Kubernetes has v1/Secret 'kube-system/secret'
    But Kubernetes doesn't have v1/Secret 'kube-public/secret'

  @create
  Scenario: Synced secret already exists
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/namespace-selector: sync=secret
    data:
      username: bXktYXBw
      password: Mzk1MjgkdmRnN0pi
    """
    And Kubernetes creates a new v1/Secret 'kube-public/secret'
    When the secret reconciler reconciles 'default/secret'
    Then Kubernetes resource v1/Secret 'kube-public/secret' is not similar to 'default/secret'
    And Kubernetes resource v1/Secret 'kube-public/secret' doesn't have label 'secret.sync.klst.pw/origin.name'
    And Kubernetes resource v1/Secret 'kube-public/secret' doesn't have label 'secret.sync.klst.pw/origin.namespace'

  @create
  Scenario: Secret has protected label
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/namespace-selector: sync=secret
      labels:
        do-not-copy: ''
    """
    And the label 'do-not-copy' is protected by the reconciler
    When the secret reconciler reconciles 'default/secret'
    Then Kubernetes resource v1/Secret 'kube-public/secret' doesn't have label 'do-not-copy'

  @create
  Scenario: Secret has protected annotation
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/namespace-selector: sync=secret
        do-not-copy: ''
    """
    And the annotation 'do-not-copy' is protected by the reconciler
    When the secret reconciler reconciles 'default/secret'
    Then Kubernetes resource v1/Secret 'kube-public/secret' doesn't have annotation 'do-not-copy'

  @update
  Scenario: Secret's content is updated
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/all-namespaces: 'true'
    data:
      username: bXktYXBw
      password: Mzk1MjgkdmRnN0pi
    """
    And the secret reconciler reconciles 'default/secret'
    When Kubernetes patches v1/Secret 'default/secret' with
    """
    data:
      username: bmVvYWRtaW4K
      group: d2h5IHlvdSBkbyB0aGF0ID8K
    """
    And Kubernetes resource v1/Secret 'kube-public/secret' is not similar to 'default/secret'
    And the secret reconciler reconciles 'default/secret'
    Then Kubernetes resource v1/Secret 'kube-public/secret' is similar to 'default/secret'
    And Kubernetes resource v1/Secret 'kube-system/secret' is equal to 'kube-public/secret'

  @update
  Scenario: Secret's annotation is updated
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/all-namespaces: 'true'
    """
    And the secret reconciler reconciles 'default/secret'
    When Kubernetes removes annotation 'secret.sync.klst.pw/all-namespaces' on v1/Secret 'default/secret'
    And Kubernetes annotates v1/Secret 'default/secret' with 'secret.sync.klst.pw/namespace-selector=sync=secret'
    And the secret reconciler reconciles 'default/secret'
    Then Kubernetes has v1/Secret 'kube-public/secret'
    But Kubernetes doesn't have v1/Secret 'kube-system/secret'

  @update
  Scenario: Secret's annotation is removed
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/all-namespaces: 'true'
    """
    And the secret reconciler reconciles 'default/secret'
    When Kubernetes removes annotation 'secret.sync.klst.pw/all-namespaces' on v1/Secret 'default/secret'
    And the secret reconciler reconciles 'default/secret'
    Then Kubernetes doesn't have v1/Secret 'kube-public/secret'
    And Kubernetes doesn't have v1/Secret 'kube-system/secret'

  @delete
  Scenario: Secret is removed
    Given Kubernetes must have v1/Secret 'default/secret' with
    """
    metadata:
      annotations:
        secret.sync.klst.pw/all-namespaces: 'true'
    """
    And the secret reconciler reconciles 'default/secret'
    When Kubernetes removes v1/Secret 'default/secret'
    And the secret reconciler reconciles 'default/secret'
    Then Kubernetes doesn't have v1/Secret 'default/secret'
# TODO: Need GC on FakeClient
#    And Kubernetes doesn't have v1/Secret 'kube-public/secret'
#    And Kubernetes doesn't have v1/Secret 'kube-system/secret'
