.ONESHELL:

build:
	go build -o $$GOPATH/bin/omni

build-watch: build
	watchexec -- make build
