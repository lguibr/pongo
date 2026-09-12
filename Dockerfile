FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o pongo .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/pongo .

# Cloud Run supplies PORT; 8080 is the default when it is unset.
EXPOSE 8080

ENTRYPOINT ["./pongo"]
