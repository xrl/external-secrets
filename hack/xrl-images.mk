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

xrl.image.build: xrl.native.check xrl.cache.digest ## Compile all providers natively, then run the image preparation hook
	$(MAKE) build-$(XRL_ARCH) PROVIDER=all_providers BUILD_ARGS=CGO_ENABLED=0 OUTPUT_DIR=bin
	file bin/external-secrets-linux-$(XRL_ARCH)
	go version -m bin/external-secrets-linux-$(XRL_ARCH)
	$(DOCKER) buildx build --load --provenance=false --sbom=false --platform linux/$(XRL_ARCH) \
		-f Dockerfile.xrl --build-arg REVISION=$(XRL_REVISION) --build-arg VERSION=$(XRL_TAG) \
		-t $(XRL_IMAGE) .

xrl.image.verify: ## Fresh non-root process, no network, no writable filesystem or mounts
	DOCKER='$(DOCKER)' bash hack/xrl-image-verify.sh '$(XRL_IMAGE)'

.PHONY: xrl.cache.test xrl.cache.provider.test xrl.cache.command.test xrl.cache.build xrl.cache.digest
xrl.cache.test: xrl.cache.provider.test xrl.cache.command.test ## Credential-free provider routing and cache CLI tests

xrl.cache.provider.test: ## Test the provider module independently, including real cache preparation
	cd providers/v1/onepasswordsdk && GOWORK=off go test -race .
xrl.cache.command.test: ## Test the cache CLI under root all-provider dependency resolution
	GOWORK=off go test -tags all_providers ./cmd/controller -run '^TestOnePasswordCacheCommand$$'

xrl.cache.build: ## Build the exact all-provider CLI locally without code generation
	GOWORK=off go build -tags all_providers -o $(OUTPUT_DIR)/external-secrets-cache .

xrl.cache.digest: ## Verify the pinned SDK still embeds the original shipping WASM
	@directory=$$(GOWORK=off go list -m -f '{{.Dir}}' github.com/1password/onepassword-sdk-go); \
	 test -n "$$directory"; \
	 python3 -c 'import hashlib, pathlib, sys; digest=hashlib.sha256(pathlib.Path(sys.argv[1], "internal/wasm/core.wasm").read_bytes()).hexdigest(); print("core.wasm sha256=" + digest); assert digest == "23d115f4ac7519b48172df3e8615945572dbda7033d51b44c9490fd533ae0f23"' "$$directory"
