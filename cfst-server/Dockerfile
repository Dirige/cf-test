FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cfst-server .

FROM alpine:3.19

ARG TARGETARCH

RUN apk --no-cache add ca-certificates tzdata curl wget tar

WORKDIR /app

COPY --from=builder /app/cfst-server .

RUN \
    wget -O /tmp/cfst.tar.gz https://github.com/XIU2/CloudflareSpeedTest/releases/download/v2.3.5/cfst_linux_${TARGETARCH}.tar.gz && \
    tar -xzf /tmp/cfst.tar.gz -C /app && \
    chmod +x /app/cfst && \
    rm -f /tmp/cfst.tar.gz

COPY --from=builder /app/config.yaml .

EXPOSE 8080

CMD ["./cfst-server", "-c", "/app/config.yaml"]
