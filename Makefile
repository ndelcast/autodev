GO := /opt/homebrew/bin/go
BINARY := autodev

.PHONY: build test lint clean docker-laravel docker-base docker-node docker-all

build:
	$(GO) build -o $(BINARY) .

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...

docker-laravel:
	docker build -t autodev-laravel images/laravel/

docker-base:
	docker build -t autodev-base images/base/

docker-node:
	docker build -t autodev-node images/node/

docker-all: docker-laravel docker-base docker-node

clean:
	rm -f $(BINARY)
	rm -f autodev.db autodev.db-wal autodev.db-shm
