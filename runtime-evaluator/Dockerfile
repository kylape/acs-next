FROM golang:1.25 AS builder
WORKDIR /app
COPY apis/ /apis/
COPY runtime-evaluator/go.mod runtime-evaluator/go.sum ./
RUN go mod download
COPY runtime-evaluator/ .
RUN CGO_ENABLED=0 go build -o /runtime-evaluator ./cmd/runtime-evaluator

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /runtime-evaluator /usr/local/bin/runtime-evaluator
ENTRYPOINT ["/usr/local/bin/runtime-evaluator"]
