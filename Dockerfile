# Build stage is not needed since GoReleaser provides the binary
# Using distroless for minimal, secure image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the binary from GoReleaser
COPY hermod /usr/local/bin/hermod

# distroless images already use nonroot user (UID 65532)
USER nonroot:nonroot

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/hermod"]
