FROM golang:1.10-alpine AS build

WORKDIR /go/src/github.com/Financial-Times/prometheus-biz-ops-service-discovery/

RUN apk add --update --no-cache curl git && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

COPY . .

RUN dep ensure && \
    go build -o /tmp/biz-ops-service-discovery cmd/biz-ops-service-discovery/main.go

FROM alpine:latest

RUN apk add --update --no-cache ca-certificates

WORKDIR /root/

COPY --from=build /tmp/biz-ops-service-discovery .

CMD ["/root/biz-ops-service-discovery"]
