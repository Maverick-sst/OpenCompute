FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/coordinator ./cmd/coordinator \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && mkdir -p /out/data && chown 65532:65532 /out/data

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/coordinator /coordinator
COPY --from=build /out/worker /worker
COPY --from=build --chown=65532:65532 /out/data /data
ENV OCF_ADDR=0.0.0.0:8080
ENV OCF_STATE_PATH=/data/ocf-state.json
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/coordinator"]
