VERSION ?= 0.1.0
DIST_DIR ?= dist
CLI := ./cmd/mtmr-lyrx
LDFLAGS := -X github.com/zakyyudha/mtmr-lyrx/internal/cli.version=$(VERSION)

.PHONY: build test swift-build release cask checksums clean

build:
	go build -ldflags "$(LDFLAGS)" -o mtmr-lyrx $(CLI)

test:
	GOPROXY=direct go test ./...

swift-build:
	swift build --package-path macos/mtmr-lyrx-menu

release: clean
	mkdir -p $(DIST_DIR)/mtmr-lyrx_darwin_arm64 $(DIST_DIR)/mtmr-lyrx_darwin_amd64
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/mtmr-lyrx_darwin_arm64/mtmr-lyrx $(CLI)
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/mtmr-lyrx_darwin_amd64/mtmr-lyrx $(CLI)
	tar -czf $(DIST_DIR)/mtmr-lyrx_darwin_arm64.tar.gz -C $(DIST_DIR)/mtmr-lyrx_darwin_arm64 mtmr-lyrx
	tar -czf $(DIST_DIR)/mtmr-lyrx_darwin_amd64.tar.gz -C $(DIST_DIR)/mtmr-lyrx_darwin_amd64 mtmr-lyrx
	$(MAKE) checksums

cask: clean
	scripts/package-cask.sh $(VERSION)

checksums:
	shasum -a 256 $(DIST_DIR)/*.tar.gz > $(DIST_DIR)/checksums.txt

clean:
	rm -rf $(DIST_DIR)
