DISTFILE=terraform-provider-flexbot
VERSION=1.2.0
OSFLAG=$(shell go env GOHOSTOS)

default: build

build:
	@go build -o $(DISTFILE)_v$(VERSION) .
	../hack/upx-${OSFLAG} $(DISTFILE)_v$(VERSION)

clean:
	@rm -f ./dist/*

dist:
	# Build for darwin-amd64
	GOOS=darwin GOARCH=amd64 go build -o dist/$(DISTFILE)_v$(VERSION).darwin
	../hack/upx-${OSFLAG} dist/$(DISTFILE)_v$(VERSION).darwin
	# Build for linux-amd64
	GOOS=linux GOARCH=amd64 go build -o ./dist/$(DISTFILE)_v$(VERSION).linux
	../hack/upx-${OSFLAG} dist/$(DISTFILE)_v$(VERSION).linux

.PHONY: dist
