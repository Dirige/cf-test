FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cfst-server .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata curl wget tar

WORKDIR /app

COPY --from=builder /app/cfst-server .
COPY --from=builder /app/config.yaml .

RUN \
    wget -O /tmp/cfst_amd64.tar.gz https://github.com/XIU2/CloudflareSpeedTest/releases/download/v2.3.5/cfst_linux_amd64.tar.gz && \
    tar -xzf /tmp/cfst_amd64.tar.gz -C /app && \
    chmod +x /app/cfst && \
    mv /app/cfst /app/cfst_amd64 && \
    rm -f /tmp/cfst_amd64.tar.gz && \
    wget -O /tmp/cfst_arm64.tar.gz https://github.com/XIU2/CloudflareSpeedTest/releases/download/v2.3.5/cfst_linux_arm64.tar.gz && \
    tar -xzf /tmp/cfst_arm64.tar.gz -C /app && \
    chmod +x /app/cfst && \
    mv /app/cfst /app/cfst_arm64 && \
    rm -f /tmp/cfst_arm64.tar.gz

EXPOSE 8080

COPY <<EOF /entrypoint.sh
#!/bin/sh
ARCH=\$(uname -m)
if [ "\$ARCH" = "aarch64" ] || [ "\$ARCH" = "arm64" ]; then
    CFST_PATH="./cfst_arm64"
else
    CFST_PATH="./cfst_amd64"
fi
sed -i "s|cfst_path: \".*\"|cfst_path: \"\$CFST_PATH\"|" /app/config.yaml
exec ./cfst-server -c /app/config.yaml
EOF
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD []
