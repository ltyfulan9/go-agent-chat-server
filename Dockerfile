ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -g '' appuser && apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/server /app/server
EXPOSE 8080
USER appuser
ENTRYPOINT ["/app/server"]
