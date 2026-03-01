FROM alpine:latest

LABEL org.opencontainers.image.source = "https://github.com/ghobs91/lodestone"
RUN ["apk", "--no-cache", "add", "ca-certificates","curl","iproute2-ss"]

COPY lodestone /usr/local/bin/lodestone
ENTRYPOINT ["/usr/local/bin/lodestone"]
