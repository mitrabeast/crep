FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o crep .

FROM alpine:latest
RUN apk --no-cache add ca-certificates docker-cli
WORKDIR /root/
COPY --from=builder /app/crep .
CMD ["./crep"]