# The binary is built by `task build` (GOOS=linux GOARCH=amd64 CGO_ENABLED=0) and copied in.
# distroless carries CA certificates, needed to reach an S3 endpoint over TLS, and runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY blobs /app/blobs

# api listens on 8080; sink and migrate-kv expose metrics on --metrics-listen-addr (9102 by convention)
EXPOSE 8080
EXPOSE 9102

ENTRYPOINT ["/app/blobs"]
CMD ["--help"]
