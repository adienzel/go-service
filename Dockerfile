# Step 1: Build the Go binary (large image)
FROM golang:1.23 AS builder

# Set the working directory inside the container
WORKDIR /app

# Clone the Go source code from GitHub (replace with your repo URL)
# Install necessary Go dependencies (modules)
# Build the Go application binary
RUN git clone https://github.com/adienzel/go-service.git . && go mod tidy && go build -o httpClient .

# Step 2: Build a minimal image with just the compiled executable
FROM alpine:latest

# Install any necessary dependencies (if required by your Go binary)
# RUN apk add --no-cache ca-certificates

# Set the working directory in the final image
WORKDIR /root/

# Copy the compiled Go binary from the build stage
COPY --from=builder /app/httpClient .

# Make the binary executable
RUN chmod +x httpClient


# define Environment Variable
# the address to send the requsts from the service
ENV SERVICE_REQUEST_HOST_ADDRES="127.0.0.1"
# The port to send the request to
ENV SERVICE_REQUEST_PORT="8980"
# the listening port where requests will come in
ENV SERVICE_LISTENING_PORT="8992"

ENV SERVICE_NUMBER_OF_CLIENTS=1
ENV SERVICE_MESAGES_PER_SECOND=1.0
ENV SERVICE_LOGLEVEL="debug"
ENV SERVICE_VERSION="V1.0"
ENV SERVICE_SCYLLA_DB_ADDRESS="127.0.0.1"
ENV SERVICE_SCYLLADB_PORT="9060"
ENV SERVICE_SCYLLADB_KEYSPACE_NAME="vin"
ENV SERVICE_SCYLLADB_REPLICATION_FACTOR=1
ENV SERVICE_SCYLLADB_STRATEGY="SimpleStrategy"
ENV SERVICE_SCYLLADB_TABLE_NAME="vehicles"

# Define the entrypoint for the container (the Go binary)
ENTRYPOINT ["./httpClient"]

# Expose any necessary ports (optional, depending on your app)
EXPOSE ${SERVICE_LISTENING_PORT}

# Default command if no args are provided
CMD []
