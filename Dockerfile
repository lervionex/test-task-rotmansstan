FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /withdrawals-service ./cmd/api

FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /withdrawals-service /app/withdrawals-service

EXPOSE 18080

ENTRYPOINT ["/app/withdrawals-service"]
