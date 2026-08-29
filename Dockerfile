# syntax=docker/dockerfile:1.7
FROM node:24-bookworm-slim AS dashboard-builder
WORKDIR /src/dashboard
COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY dashboard/ ./
ENV NEXT_PUBLIC_API_URL=""
RUN npm run build

FROM golang:1.26-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=dashboard-builder /src/dashboard/out/ ./internal/dashboard/dist/
RUN CGO_ENABLED=0 GOWORK=off go test ./... && \
    CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/skillbox ./cmd/skillbox

FROM alpine:3.22
RUN addgroup -S skillbox && adduser -S -G skillbox skillbox && \
    mkdir -p /app/configs /app/data && chown -R skillbox:skillbox /app
WORKDIR /app
COPY --from=builder /out/skillbox /app/skillbox
COPY configs/skillbox.example.yaml /app/configs/skillbox.yaml
USER skillbox
EXPOSE 8081
VOLUME ["/app/data"]
ENTRYPOINT ["/app/skillbox"]
CMD ["-config", "/app/configs/skillbox.yaml"]
