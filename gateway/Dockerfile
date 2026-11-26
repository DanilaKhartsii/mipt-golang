# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder

WORKDIR /src/gateway
COPY go.mod go.sum ./
COPY --from=ledger /go.mod /go.sum /src/ledger/
RUN go mod download

COPY . .
COPY --from=ledger /ledger.go /src/ledger/
COPY --from=ledger /internal /src/ledger/internal
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/gateway .

FROM alpine:3.23 AS runner

RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/gateway ./gateway
USER app

EXPOSE 8080
CMD ["./gateway"]
