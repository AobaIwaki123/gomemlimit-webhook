# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

# Compile static stripped binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o /workspace/bin/webhook ./cmd/webhook

FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/bin/webhook /webhook
USER 65532:65532

ENTRYPOINT ["/webhook"]
