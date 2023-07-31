FROM golang:1.20-alpine AS builder
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY . .
#RUN go build .
#
#FROM scratch
#COPY --from=builder /app/ /app
ENTRYPOINT ["go","run","server.go"]