FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /admission-controller ./cmd/admission-controller

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /admission-controller /usr/local/bin/admission-controller
EXPOSE 8443 8080
ENTRYPOINT ["/usr/local/bin/admission-controller"]
