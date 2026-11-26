# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder

WORKDIR /src/ledger
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ledger ./cmd/ledger

FROM alpine:3.23 AS runner

RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/ledger ./ledger
COPY --from=builder /src/ledger/budgets.json ./budgets.json
USER app

CMD ["./ledger"]
