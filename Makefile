SHELL=/usr/bin/env bash

all: build
.PHONY: all

unexport GOFLAGS

GOCC?=go

GOVERSION:=$(shell $(GOCC) version | tr ' ' '\n' | grep go1 | sed 's/^go//' | awk -F. '{printf "%d%03d%03d", $$1, $$2, $$3}')
GOVERSIONMIN:=$(shell cat GO_VERSION_MIN | awk -F. '{printf "%d%03d%03d", $$1, $$2, $$3}')

ifeq ($(shell expr $(GOVERSION) \< $(GOVERSIONMIN)), 1)
$(warning Your Golang version is go$(shell expr $(GOVERSION) / 1000000).$(shell expr $(GOVERSION) % 1000000 / 1000).$(shell expr $(GOVERSION) % 1000))
$(error Update Golang to version to at least $(shell cat GO_VERSION_MIN))
endif

# git modules that need to be loaded
MODULES:=

CLEAN:=
BINS:=

ifneq ($(strip $(LDFLAGS)),)
	ldflags+=-extldflags=$(LDFLAGS)
endif

GOFLAGS+=-ldflags="$(ldflags)"


## FFI

FFI_PATH:=extern/filecoin-ffi/
FFI_DEPS:=.install-filcrypto
FFI_DEPS:=$(addprefix $(FFI_PATH),$(FFI_DEPS))

$(FFI_DEPS): .filecoin-install ;

.filecoin-install: $(FFI_PATH)
	$(MAKE) -C $(FFI_PATH) $(FFI_DEPS:$(FFI_PATH)%=%)
	@touch $@

MODULES+=$(FFI_PATH)
BUILD_DEPS+=.filecoin-install
CLEAN+=.filecoin-install

ffi-version-check:
	@[[ "$$(awk '/const Version/{print $$5}' extern/filecoin-ffi/version.go)" -eq 3 ]] || (echo "FFI version mismatch, update submodules"; exit 1)
BUILD_DEPS+=ffi-version-check

.PHONY: ffi-version-check

$(MODULES): .update-modules ;
# dummy file that marks the last time modules were updated
.update-modules:
	git submodule update --init --recursive
	touch $@

# end git modules

## MAIN BINARIES

CLEAN+=.update-modules

deps: $(BUILD_DEPS)
.PHONY: deps

contract-metaallocator-cli: $(BUILD_DEPS)
	rm -f contract-metaallocator-cli
	$(GOCC) build $(GOFLAGS)

.PHONY: contract-metaallocator-cli
BINS+=contract-metaallocator-cli

build: contract-metaallocator-cli
	@[[ $$(type -P "contract-metaallocator-cli") ]] && echo "Caution: you have \
an existing lotus binary in your PATH. This may cause problems if you don't run 'sudo make install'" || true

.PHONY: build

install: install-contract-metaallocator-cli

install-contract-metaallocator-cli:
	install -C ./contract-metaallocator-cli /usr/local/bin/contract-metaallocator-cli

uninstall: uninstall-contract-metaallocator-cli
.PHONY: uninstall

uninstall-contract-metaallocator-cli:
	rm -f /usr/local/bin/contract-metaallocator-cli

buildall: $(BINS)

clean:
	rm -rf $(CLEAN) $(BINS)
	-$(MAKE) -C $(FFI_PATH) clean
.PHONY: clean

dist-clean:
	git clean -xdff
	git submodule deinit --all -f
.PHONY: dist-clean