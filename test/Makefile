KIND_VERSION	?= v0.7.0

# sources: https://stackoverflow.com/questions/18136918/how-to-get-current-relative-directory-of-your-makefile
MKFILE_PATH 	:= $(abspath $(lastword $(MAKEFILE_LIST)))
CURRENT_DIR 	:= $(patsubst %/,%,$(dir $(MKFILE_PATH)))

.PHONY: e2e
e2e: e2e_deploy e2e_test

.PHONY: e2e_deploy
e2e_deploy: KUBERNETES_VERSION-isdef
	docker build -t e2e/sync-secrets-controller $(CURRENT_DIR)/..
	kind load docker-image e2e/sync-secrets-controller --name e2e-kube-$(KUBERNETES_VERSION)
	kubectl create -f $(CURRENT_DIR)/e2e_controller.yaml
	kubectl wait --timeout -1s --for condition=Ready --namespace default --all pod

.PHONY: e2e_test
e2e_test:
	GO111MODULE="on" go test $(CURRENT_DIR)/... -v

.PHONY: kind_install
kind_install:
	GO111MODULE="on" go get sigs.k8s.io/kind@$(KIND_VERSION)

.PHONY: kind_setup
kind_setup: kind_install KUBERNETES_VERSION-isdef
	kind create cluster --name e2e-kube-$(KUBERNETES_VERSION) --image kindest/node:$(KUBERNETES_VERSION)

.PHONY: kind_cleanup
kind_cleanup: kind_install KUBERNETES_VERSION-isdef
	kind delete cluster --name e2e-kube-$(KUBERNETES_VERSION)

# sources: https://stackoverflow.com/questions/4728810/makefile-variable-as-prerequisite
%-isdef:
	@if [ -z '${${*}}' ]; then echo >&2 'environment variable $* is undefined' && exit 1; fi