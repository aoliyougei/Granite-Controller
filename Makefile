.PHONY: test race vet build image

test:
	CGO_ENABLED=0 go test ./...

race:
	CGO_ENABLED=1 go test -race ./...

vet:
	CGO_ENABLED=0 go vet ./...

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/granite-controller ./cmd/granite-controller

image:
	docker build --platform linux/amd64 -t granite-controller:1.0.0 .
