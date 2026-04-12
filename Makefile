# golink Makefile.
#
# Implements the project-side contract documented in
# ~/code/hetzner/kepler-452/docs/PROJECT-INTEGRATION.md.
#
# Targets:
#   build       - build a host-OS binary into ./bin/golink (for local dev)
#   test        - run lint + regression tests
#   lint        - run `go vet ./...` (golangci-lint upgrade tracked in #7)
#   deploy      - push to origin, then git-pull + build on kepler-452
#   logs        - tail journalctl -u golink on kepler-452
#   status      - systemctl status + recent journal on kepler-452
#   clean       - remove build artefacts

# Path to the hetzner repo. Override if it lives somewhere else.
HETZNER_REPO ?= $(HOME)/code/hetzner

# Per-app constants. Match the conventions in
# $(HETZNER_REPO)/kepler-452/docs/PROJECT-INTEGRATION.md.
APP      := golink
PORT     := 18081
MAIN_PKG := ./cmd/golink

# Host-OS binary for local development.
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
# deploy: push commits, then git-pull + build on kepler-452
# ----------------------------------------------------------------------
# The server clones on first run and pulls on subsequent runs.
# Build happens ON the server. Deployed code = committed code.
.PHONY: deploy
deploy:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "ERROR: uncommitted changes. Commit and push before deploying."; \
		exit 1; \
	fi
	@echo "==> pushing to origin"
	git push
	$(HETZNER_REPO)/kepler-452/deploy-app.sh $(APP) $(PORT)

# ----------------------------------------------------------------------
# logs: tail the running app's logs from kepler-452
# ----------------------------------------------------------------------
.PHONY: logs
logs:
	ssh tigger@kepler-452 'sudo journalctl -u $(APP) -f'

# ----------------------------------------------------------------------
# status: check service health on kepler-452
# ----------------------------------------------------------------------
.PHONY: status
status:
	ssh tigger@kepler-452 'systemctl status $(APP) --no-pager; echo; sudo journalctl -u $(APP) -n 20 --no-pager'

# ----------------------------------------------------------------------
# clean: remove build artefacts
# ----------------------------------------------------------------------
.PHONY: clean
clean:
	rm -rf ./bin
