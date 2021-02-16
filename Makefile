# Calculate version
version = $(shell ./version-at-commit.sh)

all: build

build:
	go build -ldflags "-s -w -X github.com/cure/cryptotsla/main.version=$(version)" -o cryptotsla

dev: lint
	go build -ldflags "-s -w -X github.com/cure/cryptotsla/main.version=$(version)"

lint:
	golint; golangci-lint run
	golangci-lint run

compress: build
	upx --brute cryptotsla

