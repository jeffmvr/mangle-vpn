# Build and deployment for the Go implementation of Mangle VPN.
#
# The binary is built into the parent directory so that it sits alongside
# the data and scripts directories it expects to find. The web interface in
# ui/ is compiled into the binary, so "make build" builds that first and
# the result is the only file a server needs.
#
# "make release" cross-compiles for the supported server architectures. The
# result is statically linked and has no runtime dependency on Go, so the
# server never needs a toolchain.

ROOT   := $(abspath $(CURDIR)/..)
BINARY := $(ROOT)/mangle-vpn
UI     := $(CURDIR)/ui
DIST   := $(CURDIR)/dist

GO      ?= go
LDFLAGS := -s -w

# The platforms "make release" builds for.
PLATFORMS := linux/amd64 linux/arm64

.PHONY: all
all: build


.PHONY: build
build: ui go


# Only the Go binary, embedding whatever interface build is already in
# internal/webui/dist. Quicker when only Go code has changed.
.PHONY: go
go:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/mangle


# Rebuilds the interface and the binary, then serves both on this machine
# at http://localhost:$(PORT), logging to the terminal. The first run also
# provisions ../data. Plain HTTP, because a browser will not send the Secure
# session cookie the server sets over TLS back to a non-TLS address.
PORT ?= 9443

.PHONY: run
run: build
	@test -f $(ROOT)/data/mangle.db || $(BINARY) install
	@echo "Serving on http://localhost:$(PORT)"
	$(BINARY) web -listen :$(PORT) -listen-http off -insecure -log ""


# Like run, but watches the Go code and ui/ and rebuilds and restarts the
# server on every save, through air (https://github.com/air-verse/air). An
# installed air is used when there is one; otherwise go run fetches it.
AIR ?= $(shell command -v air 2>/dev/null || echo "$(GO) run github.com/air-verse/air@v1.67.4")

.PHONY: dev
dev: build
	@test -f $(ROOT)/data/mangle.db || $(BINARY) install
	@echo "Serving on http://localhost:$(PORT), rebuilding on every save"
	PORT=$(PORT) $(AIR) -c .air.toml


# Cross-compile for every supported server platform, into dist/. Copy the
# matching binary to the server as <install root>/mangle-vpn.
.PHONY: release
release: ui
	@rm -rf $(DIST) && mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		echo "building $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GO) build -trimpath \
			-ldflags "$(LDFLAGS)" -o $(DIST)/mangle-vpn-$$os-$$arch ./cmd/mangle || exit 1; \
	done
	@echo
	@ls -lh $(DIST)


# Only the Linux amd64 build, the usual server: dist/mangle-vpn-linux-amd64.
.PHONY: dist
dist:
	@$(MAKE) --no-print-directory release PLATFORMS=linux/amd64


# Builds the web interface into internal/webui/dist, where the binary
# embeds it from.
.PHONY: ui
ui:
	cd $(UI) && npm install --no-audit --no-fund && npm run build


# The Docker image, which builds the interface and the binary itself.
# IMAGE=… to name it something else.
IMAGE ?= jeffmvr/mangle-vpn
IMAGE_PLATFORMS ?= linux/amd64,linux/arm64

# An image for this machine, loaded into the local Docker.
.PHONY: docker
docker:
	docker build -t $(IMAGE) .

# Builds the image for every platform and pushes it to Docker Hub as
# $(IMAGE):latest. Log in with "docker login" first. Building for several
# platforms at once needs a BuildKit builder of its own, which is created
# the first time and left in place.
.PHONY: docker-push
docker-push:
	docker buildx inspect mangle >/dev/null 2>&1 || docker buildx create --name mangle --driver docker-container
	docker buildx build --builder mangle --platform $(IMAGE_PLATFORMS) -t $(IMAGE):latest --push .


.PHONY: test
test:
	$(GO) test ./...


.PHONY: vet
vet:
	$(GO) vet ./...
	gofmt -l . | tee /dev/stderr | (! read)


.PHONY: tidy
tidy:
	$(GO) mod tidy


.PHONY: clean
clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
