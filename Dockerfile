# Start with a base image that includes Go
FROM golang:1.18-buster

# Install Ghostscript
RUN apt-get update && apt-get install -y ghostscript

# Set the working directory
WORKDIR /app

# Copy the Go application source code into the container
COPY . .

# Build the Go application
RUN go build -o main .

# Expose the necessary port (if your app serves HTTP requests)
EXPOSE 8080

# Start the Go program
CMD ["./main"]
