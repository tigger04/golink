# golink Makefile.
#
# Local dev targets only. Deployment is handled by the hetzner repo's
# deploy-app command — see ~/code/hetzner/deploy/docs/PROJECT-INTEGRATION.md.
#
# Targets:
#   build       - build a host-OS binary into ./bin/golink (for local dev)
#   test        - run lint + regression tests
#   lint        - run `go vet ./...` (golangci-lint upgrade tracked in #7)
#   clean       - remove build artefacts

APP          := golink
MAIN_PKG     := ./cmd/golink
BUILD_OUTPUT := ./bin/$(APP)

# ----------------------------------------------------------------------
# build: host-OS binary for local dev / smoke tests / make install
# ----------------------------------------------------------------------
.PHONY: build
build:
	@echo "==> building $(APP) for $$(go env GOOS)/$$(go env GOARCH)"
	@mkdir -p $(dir $(BUILD_OUTPUT))
	go build -o $(BUILD_OUTPUT) $(MAIN_PKG)
	@ls -la $(BUILD_OUTPUT)

# ----------------------------------------------------------------------
# test: lint + regression tests
# ----------------------------------------------------------------------
.PHONY: test
test: lint
	@echo "==> running regression tests"
	go test ./tests/regression/...

# ----------------------------------------------------------------------
# lint: go vet for now (golangci-lint upgrade tracked in issue #7)
# ----------------------------------------------------------------------
.PHONY: lint
lint:
	@echo "==> linting"
	go vet ./...

# ----------------------------------------------------------------------
# clean: remove build artefacts
# ----------------------------------------------------------------------
.PHONY: clean
clean:
	rm -rf ./bin
