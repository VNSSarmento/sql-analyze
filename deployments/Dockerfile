# ESTÁGIO 1: Build
FROM golang:1.26 AS construtor

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o sql_analyze cmd/api/main.go

# ESTÁGIO 2: Produção
FROM alpine:latest
WORKDIR /app

COPY --from=construtor /app/sql_analyze .

EXPOSE 8080

CMD ["./sql_analyze"]