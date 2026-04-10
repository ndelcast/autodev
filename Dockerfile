# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /autodev .

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates docker-cli

COPY --from=builder /autodev /usr/local/bin/autodev
COPY skills/ /app/skills/
COPY contexts/ /app/contexts/
COPY config.yaml /app/config.yaml

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["autodev"]
CMD ["serve"]
