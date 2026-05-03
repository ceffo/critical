FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o critical-api .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/critical-api .
EXPOSE 8080
CMD ["./critical-api"]
