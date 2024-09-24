## Start with a base image that includes Go
#FROM golang:1.23
#
## Install Ghostscript
#RUN apt-get update && apt-get install -y ghostscript
#
## Set the working directory
#WORKDIR /app
#
## Copy the Go application source code into the container
#COPY . .
#
## Build the Go application
#RUN go build -o main cmd/pdfinspector/main.go
#
## Expose the necessary port (if your app serves HTTP requests)
#EXPOSE 8080
#
## Start the Go program
#CMD ["./main"]

# Stage 1: Build the Go binary
FROM golang:1.23 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy only go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./

# Download dependencies (this layer will be cached unless go.mod or go.sum changes)
RUN go mod download

# Copy the entire project into the container
COPY . .

# Set the working directory to the directory where your main.go file is located
WORKDIR /app/cmd/pdfinspector

# Build the Go application statically (optional but results in a more portable binary)
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/pdfinspector .

# Stage 2: Create a smaller final image
FROM debian:bullseye-slim

# Install Ghostscript and CA certificates (runtime dependency)
RUN apt-get update && apt-get install -y ghostscript ca-certificates && apt-get clean && rm -rf /var/lib/apt/lists/*

# Set the working directory
WORKDIR /app

# Copy the compiled Go binary from the builder stage
COPY --from=builder /app/pdfinspector /app/pdfinspector
#COPY --from=builder /app/users /app/users
COPY --from=builder /app/response_templates /app/response_templates

# Expose the necessary port (if your app serves HTTP requests)
EXPOSE 8080

# Start the Go binary
CMD ["/app/pdfinspector"]