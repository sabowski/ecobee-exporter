SHELL = /bin/bash

default: build
.PHONY: clean deps build build-image push-image run

PLATFORM := $(shell uname -s)

clean:
	rm -rf build/*
	go clean --modcache

deps:
	go mod download

SRC = $(shell find . -name "*.go" | grep -v "_test\." )
build/ecobee-exporter: $(SRC)
	go build -o build/ecobee-exporter ./main.go

build: deps build/ecobee-exporter

build-image:
	docker build -t petewall/ecobee-exporter .

push-image:
	docker push petewall/ecobee-exporter

run: build/ecobee-exporter
	build/ecobee-exporter --debug --ecobeeClientId $(CLIENT_ID)