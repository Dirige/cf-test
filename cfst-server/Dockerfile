FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cfst-server .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/cfst-server .
COPY --from=builder /app/config.yaml .

EXPOSE 8080

CMD ["./cfst-server", "-c", "config.yaml"]
