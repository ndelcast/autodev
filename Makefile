GO := /opt/homebrew/bin/go
BINARY := autodev

.PHONY: build test lint clean docker

build:
	$(GO) build -o $(BINARY) .

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...

docker:
	docker build -t autodev-runner images/

clean:
	rm -f $(BINARY)
	rm -f autodev.db autodev.db-wal autodev.db-shm
