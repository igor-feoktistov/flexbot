DISTFILE=terraform-provider-flexbot
VERSION=1.1.6

default: build

build:
	@go build -o $(DISTFILE)_v$(VERSION) .

clean:
	@rm -f ./dist/*

dist:
	# Build for darwin-amd64
	GOOS=darwin GOARCH=amd64 go build -o ./dist/$(DISTFILE)_v$(VERSION).darwin
	# Build for linux-amd64
	GOOS=linux GOARCH=amd64 go build -o ./dist/$(DISTFILE)_v$(VERSION).linux

.PHONY: dist
