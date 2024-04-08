# Start with the Go base image
FROM golang:1.19 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files to the container
COPY go.mod .
COPY go.sum .

# Download the Go modules
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pongo .

# Use a minimal base image to create the final stage
FROM alpine:latest

# Add necessary CA certificates
RUN apk --no-cache add ca-certificates

# Set the working directory in the final image
WORKDIR /root/

# Copy the statically-linked binary from the builder stage
COPY --from=builder /app/pongo .

# Expose port 3001
EXPOSE 3001

# Command to run the binary
CMD ["./pongo"]
