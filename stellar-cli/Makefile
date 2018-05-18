PKG := github.com/mua69/go/stellar-cli
VERSION := $(shell git describe --always --long --dirty)

all:
	go build -i -v  -ldflags="-X main.g_gitHash=${VERSION}" ${PKG}
