@owned_secret
Feature: Secret synchronization on Owned secret's updates
  Owned secrets should always have the same value
  as the original secret.
  NOTE: Owned secret are copies of the original secret

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
          secret.sync.klst.pw/all-namespaces: 'true'
      data:
        username: bXktYXBw
        password: Mzk1MjgkdmRnN0pi
      """
    And the secret reconciler reconciles 'default/secret'

  @not_exists
  Scenario: Owned secret doesn't exists
    When the owned secret reconciler reconciles 'kube-public/secret-not-exists'
    Then nothing occurs

  @no_update
  Scenario: Owned secret already up-to-date
    When the owned secret reconciler reconciles 'kube-public/secret'
    Then nothing occurs

  @update
  Scenario: Owned secret is updated
    When Kubernetes annotates v1/Secret 'kube-public/secret' with 'modified=true'
    And the owned secret reconciler reconciles 'kube-public/secret'
    Then Kubernetes resource v1/Secret 'kube-public/secret' doesn't have annotation 'modified'

  @delete
  Scenario: Owned secret is removed
    Given Kubernetes has v1/Secret 'kube-public/secret'
    When Kubernetes removes v1/Secret 'kube-public/secret'
    And the owned secret reconciler reconciles 'kube-public/secret'
    Then Kubernetes has v1/Secret 'kube-public/secret'
    And Kubernetes resource v1/Secret 'kube-public/secret' is similar to 'default/secret'
