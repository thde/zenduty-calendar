# syntax=docker/dockerfile:1
FROM golang:1-alpine AS builder

RUN apk add -U git ca-certificates && update-ca-certificates

WORKDIR /usr/local/src/zenduty-calendar
COPY go.mod go.sum ./
RUN go mod download -x
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -trimpath -o /usr/local/bin/zenduty-calendar

FROM alpine:latest
USER "guest"

WORKDIR /tmp
HEALTHCHECK CMD wget -U "Healthcheck" --quiet --tries=1 --spider http://localhost:3000/metrics || exit 1

COPY --from=builder /usr/local/bin/zenduty-calendar /usr/local/bin/zenduty-calendar

EXPOSE 3000

ENTRYPOINT ["/usr/local/bin/zenduty-calendar"]
