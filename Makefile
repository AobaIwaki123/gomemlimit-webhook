.PHONY: all test build build-stripped docker-build clean

IMAGE ?= ghcr.io/aobaiwaki123/gomemlimit-webhook:latest

all: test build

test:
	go test -v -race ./...

build:
	CGO_ENABLED=0 go build -o bin/webhook ./cmd/webhook

build-stripped:
	CGO_ENABLED=0 go build -ldflags="-s -w -extldflags '-static'" -trimpath -o bin/webhook-stripped ./cmd/webhook

docker-build:
	docker build -t $(IMAGE) .

clean:
	rm -rf bin/
