CORE_DIR ?= .worktrees/core
CORE_REF ?= fm/roca-slop-346-playground-plugin-s1
GO := go
MOD := .tmp/build.mod

.PHONY: prepare build test acceptance check package
prepare:
	@test -f "$(CORE_DIR)/go.mod" || git clone --depth 1 --branch "$(CORE_REF)" https://github.com/thellmwhisperer/la-roca.git "$(CORE_DIR)"
	@mkdir -p .tmp bin
	@cp go.mod $(MOD)
	@cp go.sum .tmp/build.sum
	@go mod edit -modfile=$(MOD) -replace=github.com/thellmwhisperer/la-roca="$$(cd '$(CORE_DIR)' && pwd)"

build: prepare
	go build -modfile=$(MOD) -o bin/roca-playground ./cmd/roca-playground

test: prepare
	go test -modfile=$(MOD) ./...

acceptance: build
	cd "$(CORE_DIR)" && go build -o "$(CURDIR)/bin/roca" ./cmd/roca
	go test -modfile=$(MOD) -tags acceptance ./test/acceptance
	cd "$(CORE_DIR)" && ROCA_BIN="$(CURDIR)/bin/roca" ROCA_PLAYGROUND_BIN="$(CURDIR)/bin/roca-playground" ROCA_PLAYGROUND_FEATURES="$(CURDIR)/features" go test -tags acceptance ./test/acceptance -run '^TestJourneyAcceptanceSuite$$' -count=1

check: test acceptance
	go vet -modfile=$(MOD) ./...

package: build
	mkdir -p dist/package
	cp plugin.json bin/roca-playground dist/package/
	cd dist/package && shasum -a 256 plugin.json roca-playground > checksums.txt
