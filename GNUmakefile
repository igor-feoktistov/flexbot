GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
PKG_NAME=flexbot
PKG_VERSION=1.5.4
OSFLAG=$(shell go env GOHOSTOS)

default: build

build:
	go install -v ./cmd/flexbot/...

vet:
	@echo "go vet ."
	@go vet $$(go list ./... | grep -v vendor/) ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
	fi

fmt:
	gofmt -w $(GOFMT_FILES)

release:
	# Build for darwin-amd64
	@mkdir -p releases/$(PKG_VERSION)/darwin
	GOOS=darwin GOARCH=amd64 go build -o releases/$(PKG_VERSION)/darwin/$(PKG_NAME).darwin -v ./cmd/flexbot/...
	hack/upx-${OSFLAG} releases/$(PKG_VERSION)/darwin/$(PKG_NAME).darwin
	# Build for linux-amd64
	@mkdir -p releases/$(PKG_VERSION)/linux
	GOOS=linux GOARCH=amd64 go build -o releases/$(PKG_VERSION)/linux/$(PKG_NAME).linux -v ./cmd/flexbot/...
	hack/upx-${OSFLAG} releases/$(PKG_VERSION)/linux/$(PKG_NAME).linux

.PHONY: build vet fmt dist
