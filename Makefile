.PHONY: all install build

VERSION=0.1dev

all: build

build:
	go build -ldflags="-X 'main.Version=${VERSION}'" -o ./local/integra ./cmd/integra

install: build
	mv ./local/integra /usr/local/bin/integra