# golink Makefile.
#
# Local dev targets only. Deployment is handled by the hetzner repo's
# deploy-app command — see ~/code/hetzner/deploy/docs/PROJECT-INTEGRATION.md.
#
# Targets:
#   build       - build a host-OS binary into ./bin/golink (for local dev)
#   test        - run lint + regression tests
#   lint        - run `go vet ./...` (golangci-lint upgrade tracked in #7)
#   install     - build and symlink golink + goreport to ~/.local/bin
#   uninstall   - remove symlinks from ~/.local/bin
#   test-one-off - run one-off tests (optionally ISSUE=N to filter)
#   sync        - git add, commit, pull, push (including submodules)
#   release     - tag a new version (VERSION=x.y or auto-increment by 0.1)
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
# install: symlink golink binary and goreport script to ~/.local/bin
# ----------------------------------------------------------------------
.PHONY: install
install: build
	@mkdir -p $(HOME)/.local/bin
	ln -sf $(abspath $(BUILD_OUTPUT)) $(HOME)/.local/bin/golink
	ln -sf $(abspath scripts/goreport) $(HOME)/.local/bin/goreport
	@echo "==> installed golink and goreport to ~/.local/bin"

# ----------------------------------------------------------------------
# uninstall: remove symlinks from ~/.local/bin
# ----------------------------------------------------------------------
.PHONY: uninstall
uninstall:
	rm -f $(HOME)/.local/bin/golink $(HOME)/.local/bin/goreport
	@echo "==> removed golink and goreport from ~/.local/bin"

# ----------------------------------------------------------------------
# test-one-off: run one-off tests, optionally filtered by issue number
# ----------------------------------------------------------------------
.PHONY: test-one-off
test-one-off:
ifdef ISSUE
	@echo "==> running one-off tests for issue #$(ISSUE)"
	go test ./tests/one_off/... -run "$(ISSUE)"
else
	@echo "==> running all one-off tests"
	go test ./tests/one_off/...
endif

# ----------------------------------------------------------------------
# sync: git add, commit, pull, push
# ----------------------------------------------------------------------
.PHONY: sync
sync:
	git add --all
	git commit -m "sync" || true
	git pull --recurse-submodules
	git push

# ----------------------------------------------------------------------
# release: tag a new version
# ----------------------------------------------------------------------
# Usage:
#   make release              # auto-increment patch by 0.1
#   make release VERSION=1.0  # explicit version
#   SKIP_TESTS=1 make release # skip tests if already passing
.PHONY: release
release:
ifndef SKIP_TESTS
	$(MAKE) test
endif
ifndef VERSION
	$(eval LAST_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0"))
	$(eval LAST_VER := $(shell echo "$(LAST_TAG)" | sed 's/^v//'))
	$(eval VERSION := $(shell echo "$(LAST_VER) + 0.1" | bc))
endif
	@echo "==> tagging v$(VERSION)"
	git tag -a "v$(VERSION)" -m "Release v$(VERSION)"
	git push origin "v$(VERSION)"

# ----------------------------------------------------------------------
# clean: remove build artefacts
# ----------------------------------------------------------------------
.PHONY: clean
clean:
	rm -rf ./bin
