APP := html-display-server

GIT_COMMIT := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S%z)

LDFLAGS := -s -w \
	-X 'main.buildTime=$(BUILD_TIME)' \
	-X 'main.gitCommit=$(GIT_COMMIT)'

.PHONY: all build clean version

all: build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(APP) .

clean:
	rm -f $(APP)

version: build
	./$(APP) -version
