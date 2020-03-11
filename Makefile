# Copyright 2015 The Prometheus Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Ensure that 'all' is the default target otherwise it will be the first target from Makefile.common.
all::

# Needs to be defined before including Makefile.common to auto-generate targets
DOCKER_REPO             ?= xunleii
DOCKER_IMAGE_NAME       ?= sync-secrets-operator
DOCKER_ARCHS            ?= amd64 armv7 arm64 ppc64le s390x

include Makefile.common

PROMTOOL_VERSION ?= 2.5.0
PROMTOOL_URL     ?= https://github.com/prometheus/prometheus/releases/download/v$(PROMTOOL_VERSION)/prometheus-$(PROMTOOL_VERSION).$(GO_BUILD_PLATFORM).tar.gz
PROMTOOL         ?= $(FIRST_GOPATH)/bin/promtool

MACH                    ?= $(shell uname -m)

STATICCHECK_IGNORE =

# Use CGO for non-Linux builds.
ifeq ($(GOOS), linux)
	PROMU_CONF ?= .promu.yml
else
	ifndef GOOS
		ifeq ($(GOHOSTOS), linux)
			PROMU_CONF ?= .promu.yml
		else
			PROMU_CONF ?= .promu-cgo.yml
		endif
	else
		PROMU_CONF ?= .promu-cgo.yml
	endif
endif

PROMU := $(FIRST_GOPATH)/bin/promu --config $(PROMU_CONF)

.PHONY: promtool
promtool: $(PROMTOOL)

$(PROMTOOL):
	$(eval PROMTOOL_TMP := $(shell mktemp -d))
	curl -s -L $(PROMTOOL_URL) | tar -xvzf - -C $(PROMTOOL_TMP)
	mkdir -p $(FIRST_GOPATH)/bin
	cp $(PROMTOOL_TMP)/prometheus-$(PROMTOOL_VERSION).$(GO_BUILD_PLATFORM)/promtool $(FIRST_GOPATH)/bin/promtool
	rm -r $(PROMTOOL_TMP)

