# Use the official Golang image as the base image
FROM golang:1.22-alpine AS builder

# Install necessary dependencies
RUN apk update && apk add --no-cache git

# Set the working directory inside the container
WORKDIR /app

# Copy the Go modules manifest & Go modules lock files to cache dependencies
COPY go.mod go.sum ./

# Download and cache Go modules
RUN go mod download

# Copy the source code into the container
COPY main.go main.go

# Build the Go application
RUN go build -o secret-watcher .

# Use a minimal image for the final container
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the executable from the builder stage
COPY --from=builder /app/secret-watcher .

# Set the default command to run the application
CMD ["./secret-watcher"]
