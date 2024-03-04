FROM busybox:1.35.0-glibc

WORKDIR /app
COPY blob-service /app/blob-service

EXPOSE 8080
EXPOSE 9000

ENTRYPOINT ["/app/blob-service"]
