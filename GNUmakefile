GOFMT_FILES?=$$(find . -name '*.go' |grep -v vendor)
PKG_NAME=flexbot
PKG_VERSION=1.4.2
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

dist:
	GOOS=darwin GOARCH=amd64 go build -o bin/$(PKG_NAME)-v$(PKG_VERSION).darwin -v ./cmd/flexbot/...
	hack/upx-${OSFLAG} bin/$(PKG_NAME)-v$(PKG_VERSION).darwin
	GOOS=linux GOARCH=amd64 go build -o bin/$(PKG_NAME)-v$(PKG_VERSION).linux -v ./cmd/flexbot/...
	hack/upx-${OSFLAG} bin/$(PKG_NAME)-v$(PKG_VERSION).linux

.PHONY: build vet fmt dist
