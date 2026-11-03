FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/coordinator ./cmd/coordinator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/coordinator /coordinator
EXPOSE 8080
ENV OCF_ADDR=0.0.0.0:8080
ENTRYPOINT ["/coordinator"]
