# Fork-only image targets; the upstream build/image defaults remain untouched.
XRL_ARCH ?= $(shell go env GOHOSTARCH)
XRL_TAG ?= local
XRL_REVISION ?= $(shell git rev-parse HEAD)
XRL_IMAGE := ghcr.io/xrl/external-secrets:$(XRL_TAG)-$(XRL_ARCH)
ACTIONLINT ?= actionlint

.PHONY: xrl.images.test xrl.images.lint xrl.native.check xrl.image.build xrl.image.verify xrl.tools.actionlint
xrl.tools.actionlint: ## Install the pinned workflow linter into the repository-local tool directory
	GOWORK=off GOBIN=$(abspath $(LOCALBIN)) go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12

xrl.images.test: ## Test fork tags, publishing guards and digest manifest validation
	PYTHONDONTWRITEBYTECODE=1 python3 hack/xrl-images_test.py

xrl.images.lint: ## Lint the isolated fork image workflow
	$(ACTIONLINT) .github/workflows/xrl-images.yml
	bash -n hack/xrl-image-prepare.sh hack/xrl-image-verify.sh

xrl.native.check: ## Refuse cross-compilation or a toolchain different from the baseline
	test "$$(uname -s)" = Linux
	case '$(XRL_ARCH)' in amd64) test "$$(uname -m)" = x86_64 ;; arm64) test "$$(uname -m)" = aarch64 ;; *) exit 1 ;; esac
	test "$$(go env GOHOSTARCH)" = '$(XRL_ARCH)'
	test "$$(go env GOVERSION)" = go1.26.5

xrl.image.build: xrl.native.check ## Compile all providers natively, then run the image preparation hook
	$(MAKE) build-$(XRL_ARCH) PROVIDER=all_providers BUILD_ARGS=CGO_ENABLED=0 OUTPUT_DIR=bin
	file bin/external-secrets-linux-$(XRL_ARCH)
	go version -m bin/external-secrets-linux-$(XRL_ARCH)
	$(DOCKER) buildx build --load --provenance=false --sbom=false --platform linux/$(XRL_ARCH) \
		-f Dockerfile.xrl --build-arg REVISION=$(XRL_REVISION) --build-arg VERSION=$(XRL_TAG) \
		-t $(XRL_IMAGE) .

xrl.image.verify: ## Fresh non-root process, no network, no writable filesystem or mounts
	DOCKER='$(DOCKER)' bash hack/xrl-image-verify.sh '$(XRL_IMAGE)'
