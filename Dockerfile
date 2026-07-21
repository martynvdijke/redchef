# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o redchef .

# Stage 2: Runtime
FROM alpine:3.24

RUN apk add --no-cache wget ffmpeg
WORKDIR /app
COPY --from=builder /build/redchef .
COPY --from=builder /build/static ./static
EXPOSE 6270
VOLUME ["/app/media", "/db"]
ENV PORT=6270
ENTRYPOINT ["/app/redchef"]
