FROM golang:1.10-alpine AS build

WORKDIR /go/src/github.com/Financial-Times/prometheus-health-check-exporter/

RUN apk add --update --no-cache curl git && \
    curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

COPY . .

RUN dep ensure && \
    go build -o /tmp/health-check-exporter cmd/health-check-exporter/main.go

FROM alpine:latest

RUN apk add --update --no-cache ca-certificates

WORKDIR /root/

COPY --from=build /tmp/health-check-exporter .

CMD ["/root/biz-ops-service-discovery"]
