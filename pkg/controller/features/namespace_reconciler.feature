@namespace
Feature: Secret synchronization on Namespace's updates
  Secrets should always be synchronized when
  a Namespace is created, updated or removed.

  Background:
    Given Kubernetes must have the following resources
      | ApiGroupVersion | Kind      | Namespace | Name        |
      | v1              | Namespace |           | kube-system |
      | v1              | Namespace |           | kube-public |
      | v1              | Namespace |           | default     |
    And Kubernetes labelizes v1/Namespace 'kube-public' with 'sync=secret'
    And Kubernetes creates a new v1/Secret 'default/secret' with
      """
      metadata:
        annotations:
          secret.sync.klst.pw/namespace-selector: 'sync=secret'
      data:
        username: bXktYXBw
        password: Mzk1MjgkdmRnN0pi
      """
    And the secret reconciler reconciles 'default/secret'

  @create
  Scenario: Namespace created without labels
    When Kubernetes creates a new v1/Namespace 'kubetest'
    And the namespace reconciler reconciles 'kubetest'
    Then Kubernetes doesn't have v1/Secret 'kubetest/secret'

  @create
  Scenario: Namespace created with 'sync=secret' label
    When Kubernetes creates a new v1/Namespace 'kubetest' with
    """
    metadata:
      labels:
        sync: 'secret'
    """
    And the namespace reconciler reconciles 'kubetest'
    Then Kubernetes has v1/Secret 'kubetest/secret'

  @update
  Scenario: Namespace is updated with the selector label
    When Kubernetes creates a new v1/Namespace 'kubetest' with
    """
    metadata:
      labels:
        wrong-label: 'true'
    """
    And the namespace reconciler reconciles 'kubetest'
    And Kubernetes labelizes v1/Namespace 'kubetest' with 'sync=secret'
    And the namespace reconciler reconciles 'kubetest'
    Then Kubernetes has v1/Secret 'kubetest/secret'

  @update
  Scenario: Namespace is updated by removing the selector label
    When Kubernetes creates a new v1/Namespace 'kubetest' with
    """
    metadata:
      labels:
        sync: 'secret'
    """
    And the namespace reconciler reconciles 'kubetest'
    And Kubernetes removes label 'sync' on v1/Namespace 'kubetest'
    And the namespace reconciler reconciles 'kubetest'
    Then Kubernetes doesn't have v1/Secret 'kubetest/secret'

  @update
  Scenario: Namespace is updated by editing the selector label
    When Kubernetes creates a new v1/Namespace 'kubetest' with
    """
    metadata:
      labels:
        sync: 'secret'
    """
    And the namespace reconciler reconciles 'kubetest'
    And Kubernetes updates label 'sync' on v1/Namespace 'kubetest' with 'false'
    And the namespace reconciler reconciles 'kubetest'
    Then Kubernetes doesn't have v1/Secret 'kubetest/secret'

  @delete
  Scenario: Namespace is removed
    When Kubernetes creates a new v1/Namespace 'kubetest' with
    """
    metadata:
      labels:
        sync: 'secret'
    """
    And the namespace reconciler reconciles 'kubetest'
    And Kubernetes removes v1/Namespace 'kubetest'
    And the namespace reconciler reconciles 'kubetest'
    Then Kubernetes doesn't have v1/Secret 'kubetest/secret'
