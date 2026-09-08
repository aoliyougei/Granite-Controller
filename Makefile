.PHONY: test race vet build native-test image

test:
	CGO_ENABLED=0 go test ./...

race:
	CGO_ENABLED=1 go test -race ./...

vet:
	CGO_ENABLED=0 go vet ./...

build:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/needle-controller ./cmd/needle-controller

native-test:
	CGO_ENABLED=1 go test -tags=needle_native ./internal/native -run TestRealNeedle -count=1

image:
	docker build --platform linux/amd64 -t needle-controller:0.2.0 .
