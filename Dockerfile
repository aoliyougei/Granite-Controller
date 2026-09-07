FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/needle-controller ./cmd/needle-controller

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/needle-controller /needle-controller
COPY etc/needle-controller.yaml /etc/needle-controller/config.yaml

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/needle-controller"]
CMD ["-f", "/etc/needle-controller/config.yaml"]
