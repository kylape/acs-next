FROM golang:1.23 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /broker ./cmd/broker

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /broker /usr/local/bin/broker
RUN mkdir -p /data/jetstream
EXPOSE 4222 8080
ENTRYPOINT ["/usr/local/bin/broker"]
