FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /livelink-hub .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /livelink-hub /app/livelink-hub
RUN mkdir -p /data /data/recordings
EXPOSE 8790/tcp 21101/udp 21101/tcp 9990/tcp 1935/tcp 8554/tcp 8888/tcp 8889/tcp
ENV LIVELINK_WEB_ADDR=0.0.0.0:8790
ENTRYPOINT ["/app/livelink-hub", "-headless", "-config", "/data/config.json", "-log", "-"]
