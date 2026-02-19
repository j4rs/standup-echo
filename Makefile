VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w \
           -X github.com/j4rs/standup-echo/cmd.version=$(VERSION) \
           -X github.com/j4rs/standup-echo/cmd.commit=$(COMMIT) \
           -X github.com/j4rs/standup-echo/cmd.date=$(DATE)

.PHONY: build install test clean run

build:
	go build -ldflags "$(LDFLAGS)" -o bin/standup-echo .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

clean:
	rm -rf bin/

run: build
	./bin/standup-echo serve
