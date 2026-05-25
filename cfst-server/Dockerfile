FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cfst-server .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata curl wget unzip

WORKDIR /app

COPY --from=builder /app/cfst-server .
COPY --from=builder /app/config.yaml .

RUN wget -O ./cfst https://github.com/Dirige/CloudflareSpeedTest/releases/latest/download/CloudflareST_linux_amd64.zip && \
    unzip -o ./cfst && \
    chmod +x ./cfst && \
    rm -f ./cfst

EXPOSE 8080

CMD ["./cfst-server", "-c", "config.yaml"]
