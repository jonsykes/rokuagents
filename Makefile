# managents — developer entry points. Run `make help` for the list.

PIO      ?= pio
OPENSCAD ?= openscad
PORT     ?=

# One version for the helper and the firmware: the latest v* tag (X.Y.Z), or X.Y.Z-N-gHASH after it,
# or a bare commit hash before the first tag. The release workflow passes VERSION=X.Y.Z from the tag.
VERSION ?= $(patsubst v%,%,$(shell git describe --tags --match 'v*' --always --dirty 2>/dev/null || echo dev))

CPP_SOURCES = $(shell find firmware/src firmware/lib firmware/test firmware/sim \
                -name '*.cpp' -o -name '*.hpp' -o -name '*.h')
STL_PARTS   = body bezel

# The merged firmware image (flash at 0x0) and its version, as `pio run -e e32r40t` writes them.
FW_NAME  = managents-firmware-e32r40t
FW_BUILD = firmware/.pio/build/e32r40t
# The helper embeds them (see helper/internal/firmware) so that `managents flash` needs nothing else.
FW_EMBED = helper/internal/firmware/images

GO_BUILD = go build -trimpath -ldflags '-s -w -X main.version=$(VERSION)'
DIST     = dist
# The user's own install location, for `make install` and for stopping the service around `make flash`.
BIN_DIR ?= $(HOME)/.local/bin

# The CHANGELOG.md section of VERSION, without its heading and the link references after the last one.
RELEASE_NOTES = awk -v heading='\#\# [$(VERSION)]' \
	'index($$0, heading) == 1 {on = 1; next} /^\#\# / || /^\[[^]]+\]: / {on = 0} on' CHANGELOG.md

# OpenSCAD snapshots render far faster with the Manifold backend; the stable
# release (2021.01) has only CGAL and no --backend option.
OPENSCAD_HAS_BACKEND = $(shell $(OPENSCAD) --help 2>&1 | grep -q -- --backend && echo yes)
OPENSCAD_FLAGS ?= $(if $(OPENSCAD_HAS_BACKEND),--backend=manifold)
OPENSCAD_CGAL   = $(if $(OPENSCAD_HAS_BACKEND),--backend=cgal)
OPENSCAD_PNG    = $(OPENSCAD) $(OPENSCAD_FLAGS) --colorscheme="Nord Light" --projection=p

.DEFAULT_GOAL := help
.PHONY: help test test-helper test-firmware lint lint-helper lint-shell format format-check \
        build helper embed-firmware firmware screens install flash monitor run \
        dist dist-check check-version release-notes enclosure enclosure-check enclosure-images clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: test-helper test-firmware ## Run every test that needs no hardware

test-helper: ## Helper unit and contract tests
	cd helper && go test -race ./...

test-firmware: ## Firmware core unit and contract tests, on the host
	cd firmware && $(PIO) test -e native

lint: lint-helper lint-shell format-check ## Static checks for helper, install script and firmware

lint-helper:
	cd helper && test -z "$$(gofmt -l .)" || (gofmt -l . && false)
	cd helper && go vet ./...
	cd helper && go tool staticcheck ./...
	cd helper && go mod tidy -diff
	cd helper && go tool govulncheck ./...

lint-shell:
	shellcheck install.sh

format: ## Format all sources in place
	cd helper && gofmt -w .
	clang-format -i $(CPP_SOURCES)

format-check:
	clang-format --dry-run --Werror $(CPP_SOURCES)

build: firmware helper ## Build the helper binary and the firmware image

helper: embed-firmware ## Build bin/managents (with the firmware image, if `make firmware` built one)
	cd helper && $(GO_BUILD) -o ../bin/managents ./cmd/managents

embed-firmware:
	@if [ -f $(FW_BUILD)/$(FW_NAME).bin ]; then \
		mkdir -p $(FW_EMBED) && \
		cp $(FW_BUILD)/$(FW_NAME).bin $(FW_BUILD)/$(FW_NAME).version $(FW_EMBED)/ && \
		echo "embedded firmware $$(cat $(FW_EMBED)/$(FW_NAME).version)"; \
	else \
		echo "no firmware image to embed: 'managents flash' will be unavailable (run make firmware first)"; \
	fi

firmware: ## Build the firmware image
	cd firmware && $(PIO) run -e e32r40t

screens: ## Render the display UI into docs/screens and check the strip painting
	rm -f docs/screens/*.png
	cd firmware && $(PIO) run -e sim && .pio/build/sim/program ../docs/screens

# Users flash with `managents flash`; this is the developer path. The service holds the port, so it
# is stopped for the upload (the leading - tolerates there being no service) and started again.
flash: ## Build and flash the firmware with PlatformIO (PORT=... to choose the serial port)
	-$(BIN_DIR)/managents service stop >/dev/null 2>&1
	cd firmware && $(PIO) run -e e32r40t -t upload $(if $(PORT),--upload-port $(PORT)); status=$$?; \
		$(BIN_DIR)/managents service start >/dev/null 2>&1; exit $$status

monitor: ## Open a serial monitor on the display
	cd firmware && $(PIO) device monitor $(if $(PORT),--port $(PORT))

install: helper ## Install bin/managents into ~/.local/bin and register the service
	mkdir -p $(BIN_DIR)
	tmp=$$(mktemp $(BIN_DIR)/.managents.XXXXXX) && cp bin/managents "$$tmp" && chmod 0755 "$$tmp" && \
		mv -f "$$tmp" $(BIN_DIR)/managents
	$(BIN_DIR)/managents service install

run: helper ## Build and run the helper
	bin/managents run $(if $(PORT),--port $(PORT))

# Release assets, built on macOS (lipo, codesign and the cgo build of the darwin helper need it).
# The names carry no version, so releases/latest/download/<asset> is a stable URL, which install.sh
# relies on. The version is stamped into the helper and the firmware alike.
STAGE = $(DIST)/stage

# $(call build-portable,GOOS,GOARCH,executable): the helpers that need no cgo.
define build-portable
cd helper && CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) $(GO_BUILD) -o ../$(STAGE)/$(1)_$(2)/$(3) ./cmd/managents
endef

# $(call archive,platform,executable,tar|zip): the binary, LICENSE and THIRD_PARTY_NOTICES.md.
define archive
cp LICENSE THIRD_PARTY_NOTICES.md $(STAGE)/$(1)/ && cd $(STAGE)/$(1) && \
	$(if $(filter zip,$(3)),zip -q ../../managents_$(1).zip,COPYFILE_DISABLE=1 tar --uid 0 --gid 0 -czf ../../managents_$(1).tar.gz) \
	$(2) LICENSE THIRD_PARTY_NOTICES.md
endef

dist: ## Build the release assets into dist/ (macOS only)
	@[ "$$(uname -s)" = Darwin ] || { echo "dist: needs macOS (cgo for the darwin helper, lipo, codesign)"; exit 1; }
	rm -rf $(DIST)
	mkdir -p $(addprefix $(STAGE)/,darwin_arm64 darwin_amd64 darwin_universal linux_amd64 linux_arm64 windows_amd64)
	MANAGENTS_VERSION=$(VERSION) $(MAKE) firmware embed-firmware
	cd helper && for arch in arm64 amd64; do \
		CGO_ENABLED=1 GOOS=darwin GOARCH=$$arch $(GO_BUILD) -o ../$(STAGE)/darwin_$$arch/managents ./cmd/managents || exit 1; \
	done
	lipo -create -output $(STAGE)/darwin_universal/managents $(STAGE)/darwin_arm64/managents $(STAGE)/darwin_amd64/managents
	codesign -s - -f -i com.github.tonylook.managents $(STAGE)/darwin_universal/managents
	$(call build-portable,linux,amd64,managents)
	$(call build-portable,linux,arm64,managents)
	$(call build-portable,windows,amd64,managents.exe)
	$(call archive,darwin_universal,managents,tar)
	$(call archive,linux_amd64,managents,tar)
	$(call archive,linux_arm64,managents,tar)
	$(call archive,windows_amd64,managents.exe,zip)
	cp $(FW_BUILD)/$(FW_NAME).bin install.sh $(DIST)/
	rm -rf $(STAGE)
	cd $(DIST) && shasum -a 256 managents_* $(FW_NAME).bin install.sh > checksums.txt

# What the release workflow checks before it publishes, and what you can run after `make dist`.
dist-check: ## Smoke-test the assets in dist/ (macOS only)
	@set -eu; \
	tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	fail() { echo "dist-check: $$*" >&2; exit 1; }; \
	(cd $(DIST) && shasum -a 256 -c checksums.txt >/dev/null) || fail "checksums.txt does not match the assets"; \
	tar -xzf $(DIST)/managents_darwin_universal.tar.gz -C "$$tmp"; \
	[ "$$("$$tmp/managents" version)" = "managents $(VERSION)" ] || fail "managents version is not $(VERSION)"; \
	[ "$$(lipo -archs "$$tmp/managents" | tr ' ' '\n' | sort | paste -sd, -)" = arm64,x86_64 ] || fail "the binary is not arm64 + x86_64"; \
	codesign --verify --all-architectures "$$tmp/managents" || fail "the binary is not signed"; \
	image=$(DIST)/$(FW_NAME).bin; size=$$(wc -c < $$image); \
	[ "$$size" -ge 300000 ] && [ "$$size" -le 4194304 ] || fail "firmware image has $$size bytes"; \
	[ "$$(od -An -tx1 -j4096 -N1 $$image | tr -d ' ')" = e9 ] || fail "firmware image has no bootloader at 0x1000"; \
	grep -aqF "$(VERSION)" $$image || fail "firmware image does not carry version $(VERSION)"; \
	for run in install upgrade; do \
		MANAGENTS_BASE_URL="file://$$PWD/$(DIST)" MANAGENTS_NO_SERVICE=1 MANAGENTS_BIN_DIR="$$tmp/bin" \
			sh $(DIST)/install.sh >/dev/null || fail "install.sh failed ($$run)"; \
	done; \
	"$$tmp/bin/managents" flash -h >/dev/null 2>&1 || fail "managents flash -h failed"; \
	echo "dist-check: $(DIST) is good"

# Releases move in lockstep with CHANGELOG.md: no section for VERSION, no release.
check-version: ## Fail unless CHANGELOG.md has a non-empty section for VERSION
	@[ -n "$$($(RELEASE_NOTES))" ] || { echo "check-version: CHANGELOG.md has no '## [$(VERSION)]' section with content"; exit 1; }

release-notes: ## Print the CHANGELOG.md section for VERSION (the text of a release)
	@$(RELEASE_NOTES)

enclosure: ## Render the enclosure STLs (needs OpenSCAD)
	$(foreach part,$(STL_PARTS),$(OPENSCAD) $(OPENSCAD_FLAGS) --export-format=binstl -D 'part="$(part)"' \
		-o enclosure/stl/managents-$(part).stl enclosure/managents_case.scad &&) true

# CGAL counts the volumes of a render: one solid and the space around it make 2.
enclosure-check: ## Check that each printed part renders as one solid
	@tmp=$$(mktemp -d) && trap 'rm -rf "$$tmp"' EXIT && \
	for part in $(STL_PARTS); do \
		$(OPENSCAD) $(OPENSCAD_CGAL) -D "part=\"$$part\"" -o "$$tmp/$$part.stl" enclosure/managents_case.scad \
			>"$$tmp/log" 2>&1 || { cat "$$tmp/log"; exit 1; }; \
		volumes=$$(sed -n 's/^[[:space:]]*Volumes:[[:space:]]*\([0-9][0-9]*\)[[:space:]]*$$/\1/p' "$$tmp/log"); \
		[ "$$volumes" = 2 ] || { echo "enclosure-check: $$part has $${volumes:-no} CGAL volumes, want 2"; exit 1; }; \
		echo "enclosure-check: $$part is one solid"; \
	done

# Cameras are translate_x,y,z,rot_x,y,z,distance, recorded so renders stay comparable.
# The Nord Light colour scheme is newer than OpenSCAD 2021.01.
enclosure-images: ## Re-render enclosure/images (needs an OpenSCAD snapshot with OpenGL)
	$(OPENSCAD_PNG) --camera=56,42,16,62,0,25,340 --imgsize=1600,1000 \
		-o enclosure/images/assembly.png enclosure/managents_case.scad
	$(OPENSCAD_PNG) --camera=64,40,20,72,0,125,320 --imgsize=1600,1000 \
		-o enclosure/images/assembly-side.png enclosure/managents_case.scad
	$(OPENSCAD_PNG) --render -D 'part="body"' --camera=60,44,14,50,0,335,360 --imgsize=1200,900 \
		-o enclosure/images/body.png enclosure/managents_case.scad
	$(OPENSCAD_PNG) --render -D 'part="bezel"' --camera=58,32,-1,45,0,20,320 --imgsize=1200,900 \
		-o enclosure/images/bezel-print.png enclosure/managents_case.scad

clean: ## Remove build outputs
	rm -rf bin $(DIST) firmware/.pio $(FW_EMBED)/$(FW_NAME).*
