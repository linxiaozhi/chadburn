FROM golang:1.20-alpine AS builder

LABEL org.opencontainers.image.description Cron alternative for Docker Swarm enviornments.

RUN if [ "mirrors.ustc.edu.cn" != "" ] ; then sed -i "s/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g" /etc/apk/repositories ; fi
RUN go env -w GOPROXY=https://proxy.golang.com.cn,https://goproxy.cn,direct

RUN apk --no-cache add gcc musl-dev

WORKDIR ${GOPATH}/src/github.com/PremoWeb/Chadburn
COPY . ${GOPATH}/src/github.com/PremoWeb/Chadburn

RUN go build -o /go/bin/chadburn .

FROM alpine

RUN if [ "mirrors.ustc.edu.cn" != "" ] ; then sed -i "s/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g" /etc/apk/repositories ; fi

RUN apk --update --no-cache add ca-certificates tzdata

COPY --from=builder /go/bin/chadburn /usr/bin/chadburn

ENTRYPOINT ["/usr/bin/chadburn"]

CMD ["daemon"]