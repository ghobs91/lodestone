FROM golang:1.23.6-alpine3.20 AS build

RUN apk --update add \
    gcc \
    musl-dev \
    git

RUN mkdir /build

COPY . /build

WORKDIR /build

RUN go build -ldflags "-s -w -X github.com/ghobs91/lodestone/internal/version.GitTag=$(git describe --tags --always --dirty)"

FROM alpine:3.20

RUN apk --update add \
    curl \
    iproute2-ss \
    && rm -rf /var/cache/apk/*

COPY --from=build /build/lodestone /usr/bin/lodestone

ENTRYPOINT ["lodestone"]
