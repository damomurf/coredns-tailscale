FROM golang:1.26.4-alpine3.23 AS build

WORKDIR /go/src/coredns

RUN apk add git make && \
    git clone --depth 1 --branch=v1.14.2 https://github.com/coredns/coredns /go/src/coredns && cd plugin

COPY . /go/src/coredns/plugin/tailscale

RUN cd plugin && \
    sed -i s/forward:forward/tailscale:tailscale\\nforward:forward/ /go/src/coredns/plugin.cfg && \
    cat /go/src/coredns/plugin.cfg && \
    cd .. && \
    go mod edit -replace github.com/coredns/coredns/plugin/tailscale=./plugin/tailscale && \
    make check && \
    go build

FROM alpine:3.23.4
RUN apk add --no-cache ca-certificates

COPY --from=build /go/src/coredns/coredns /
COPY Corefile run.sh /

ENTRYPOINT ["/run.sh"]
