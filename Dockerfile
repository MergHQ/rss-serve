FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /rss-simple ./src

FROM alpine:latest

WORKDIR /app

COPY --from=builder /rss-simple /app/rss-simple
COPY --from=builder /app/src/templates /app/src/templates

EXPOSE 3000

CMD ["./rss-simple"]