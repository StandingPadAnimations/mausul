# --- Stage 1: Build the static binary ---
FROM golang:1.26-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /webmentions ./src

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /data

# Copy the compiled binary from the builder stage
COPY --from=builder /webmentions /usr/local/bin/webmentions

USER nonroot:nonroot
EXPOSE 8000
VOLUME ["/webmentions"]
ENTRYPOINT ["/usr/local/bin/webmentions"]
