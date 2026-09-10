FROM python:3.13-slim-bookworm AS model
WORKDIR /fetch
COPY build/granite-model.env build/fetch_granite.py ./
RUN set -eu; . ./granite-model.env; \
  python fetch_granite.py \
    --source "https://huggingface.co/${GRANITE_REPOSITORY}/resolve/${GRANITE_REVISION}/${GRANITE_FILENAME}" \
    --sha256 "$GRANITE_SHA256" --out "/${GRANITE_FILENAME}"

FROM debian:bookworm-slim AS llama
ARG LLAMA_CPP_COMMIT=22397c31a00e78f55ae556c41fc78b717c5911bd
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates cmake g++ git ninja-build \
 && rm -rf /var/lib/apt/lists/*
RUN git -c http.version=HTTP/1.1 clone --filter=blob:none https://github.com/ggml-org/llama.cpp.git /src \
 && cd /src && git checkout "$LLAMA_CPP_COMMIT"
RUN cmake -S /src -B /src/build -G Ninja -DGGML_NATIVE=OFF -DGGML_CPU_ALL_VARIANTS=OFF \
      -DLLAMA_CURL=OFF -DLLAMA_BUILD_TESTS=OFF -DLLAMA_BUILD_EXAMPLES=OFF \
      -DLLAMA_BUILD_UI=OFF -DLLAMA_USE_PREBUILT_UI=OFF \
      -DLLAMA_BUILD_SERVER=ON -DCMAKE_BUILD_TYPE=Release \
 && cmake --build /src/build --target llama-server -j4 \
 && sha256sum /src/build/bin/llama-server

FROM golang:1.24-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN GODEBUG=netdns=cgo go mod download
COPY . .
RUN CGO_ENABLED=0 go test ./... \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/granite-controller ./cmd/granite-controller

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends libgomp1 \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --uid 65532 --home /nonexistent --shell /usr/sbin/nologin granite
COPY --from=go-builder /out/granite-controller /granite-controller
COPY --from=llama /src/build/bin/llama-server /src/build/bin/lib*.so* /opt/granite/
COPY --from=model /granite-4.0-350m-Q4_K_M.gguf /opt/granite/granite-4.0-350m-Q4_K_M.gguf
COPY etc/granite-controller.yaml /etc/granite-controller/config.yaml
COPY licenses /licenses
ENV LD_LIBRARY_PATH=/opt/granite
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/granite-controller"]
CMD ["-f", "/etc/granite-controller/config.yaml"]
