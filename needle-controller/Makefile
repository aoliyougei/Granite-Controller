.PHONY: test race vet build image

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

build:
	CGO_ENABLED=0 go build -trimpath -o bin/needle-controller ./cmd/needle-controller

image:
	docker build -t needle-controller:0.1.0 .
