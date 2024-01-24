FROM busybox:1.35.0-glibc

WORKDIR /app
COPY golang-service-template /app/golang-service-template

EXPOSE 8080
EXPOSE 9000

ENTRYPOINT ["/app/golang-service-template"]
