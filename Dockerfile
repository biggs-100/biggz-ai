FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /build/biggz ./cmd/biggz && \
    CGO_ENABLED=0 go build -o /build/biggz-mcp ./cmd/biggz-mcp

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git openssh-client
COPY --from=builder /build/biggz /usr/local/bin/biggz
COPY --from=builder /build/biggz-mcp /usr/local/bin/biggz-mcp
ENTRYPOINT ["biggz"]
CMD ["--help"]
