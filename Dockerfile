FROM python:3.12-slim AS needle-fetcher

WORKDIR /fetch
COPY build/needle-engine.env build/fetch_needle.py ./
RUN set -eu; \
    . ./needle-engine.env; \
    url="https://huggingface.co/${NEEDLE_HF_REPO}/resolve/${NEEDLE_HF_REVISION}/${NEEDLE_WHEEL_PATH}?download=true"; \
    python ./fetch_needle.py \
      --source "$url" \
      --wheel-sha256 "$NEEDLE_WHEEL_SHA256" \
      --lib-sha256 "$NEEDLE_LIB_SHA256" \
      --out /out

FROM golang:1.24-bookworm AS builder

WORKDIR /src
COPY --from=needle-fetcher /out/libneedle.so /opt/needle/libneedle.so
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
      go test -tags=needle_native ./internal/native -run TestRealNeedle -count=1 \
 && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
      go build -trimpath -ldflags="-s -w" -o /out/needle-controller ./cmd/needle-controller \
 && ldd /out/needle-controller \
 && readelf -d /out/needle-controller | grep -F '/opt/needle'

FROM gcr.io/distroless/cc-debian12:nonroot

COPY --from=builder /out/needle-controller /needle-controller
COPY --from=needle-fetcher /out/libneedle.so /opt/needle/libneedle.so
COPY etc/needle-controller.yaml /etc/needle-controller/config.yaml
COPY licenses/Apache-2.0.txt /licenses/needle/Apache-2.0.txt
COPY licenses/THIRD_PARTY_NOTICES.md /licenses/needle/THIRD_PARTY_NOTICES.md

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/needle-controller"]
CMD ["-f", "/etc/needle-controller/config.yaml"]
