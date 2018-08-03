ARG GO_VERSION=1.10

FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /go/src/github.com/Financial-Times/prometheus-biz-ops-service-discovery/

RUN apk add --update --no-cache curl git && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

COPY Gopkg.toml Gopkg.lock ./

RUN dep ensure -vendor-only

COPY . ./

RUN go build -o /tmp/biz-ops-service-discovery cmd/biz-ops-service-discovery/main.go

FROM alpine:latest

RUN apk add --update --no-cache ca-certificates

WORKDIR /root/

COPY --from=build /tmp/biz-ops-service-discovery .

ARG BUILD_DATE
ARG BUILD_NUMBER
ARG VCS_SHA

LABEL maintainer="reliability.engineering@ft.com" \
    com.ft.build-number="$BUILD_NUMBER" \
    org.opencontainers.authors="reliability.engineering@ft.com" \
    org.opencontainers.created="$BUILD_DATE" \
    org.opencontainers.licenses="MIT" \
    org.opencontainers.revision="$VCS_SHA" \
    org.opencontainers.title="prometheus-biz-ops-service-discovery" \
    org.opencontainers.source="https://github.com/Financial-Times/prometheus-biz-ops-service-discovery" \
    org.opencontainers.url="https://dewey.in.ft.com/view/system/prometheus-biz-ops-service-discovery" \
    org.opencontainers.vendor="financial-times"

ENTRYPOINT ["/root/biz-ops-service-discovery"]
