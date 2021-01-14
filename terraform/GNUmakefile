DISTFILE=terraform-provider-flexbot
VERSION=1.6.2
OSFLAG=$(shell go env GOHOSTOS)

default: build

build:
	@go build -o $(DISTFILE)_v$(VERSION) .
	../hack/upx-${OSFLAG} $(DISTFILE)_v$(VERSION)

clean:
	@rm -f dist/*

dist:
	# Build for darwin-amd64
	GOOS=darwin GOARCH=amd64 go build -o dist/$(DISTFILE)_v$(VERSION).darwin_amd64
	../hack/upx-${OSFLAG} dist/$(DISTFILE)_v$(VERSION).darwin_amd64
	# Build for linux-amd64
	GOOS=linux GOARCH=amd64 go build -o dist/$(DISTFILE)_v$(VERSION).linux_amd64
	../hack/upx-${OSFLAG} dist/$(DISTFILE)_v$(VERSION).linux_amd64
	# Build for linux-arm64
	GOOS=linux GOARCH=arm64 go build -o dist/$(DISTFILE)_v$(VERSION).linux_arm64
	../hack/upx-${OSFLAG} dist/$(DISTFILE)_v$(VERSION).linux_arm64

.PHONY: dist
