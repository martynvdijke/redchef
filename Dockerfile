# Stage 1: Build
FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o redchef .

# Stage 2: Runtime
FROM scratch
WORKDIR /app
COPY --from=builder /build/redchef .
COPY --from=builder /build/static ./static
EXPOSE 8080
VOLUME ["/app/uploads"]
ENV PORT=8080 DB_PATH=/app/redchef.db
ENTRYPOINT ["/app/redchef"]
