# syntax=docker/dockerfile:1

FROM golang:1.20

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/engine/reference/builder/#copy
COPY *.go ./
ADD internal ./internal

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /zenduty-calendar

EXPOSE 3000

# Run
CMD ["/zenduty-calendar"]
