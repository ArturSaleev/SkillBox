# syntax=docker/dockerfile:1.7
FROM golang:1.26-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOWORK=off go test ./... && \
    CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/skillbox ./cmd/skillbox

FROM alpine:3.22
RUN addgroup -S skillbox && adduser -S -G skillbox skillbox && \
    mkdir -p /app/configs /data && chown -R skillbox:skillbox /app /data
WORKDIR /app
COPY --from=builder /out/skillbox /app/skillbox
COPY configs/skillbox.example.yaml /app/configs/skillbox.yaml
USER skillbox
EXPOSE 8080
VOLUME ["/data"]
ENV SKILLBOX_DATABASE_PATH=/data/skillbox.db
ENTRYPOINT ["/app/skillbox"]
CMD ["-config", "/app/configs/skillbox.yaml"]
