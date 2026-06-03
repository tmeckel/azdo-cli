
FROM golang:1.26.4

WORKDIR /go/src/github.com/samber/lo

COPY Makefile go.* ./

RUN make tools
