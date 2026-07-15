FROM golang:1.22-alpine AS builder

RUN apk update && apk upgrade && apk add --no-cache make git

ENV GOPATH=/go
ENV PATH=/go/bin:$PATH

RUN go install golang.org/x/tools/cmd/goimports@v0.24.0
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN make build

FROM alpine:3.20

RUN apk update && apk upgrade && apk add --no-cache --update \
    go \
    make \
    git \
    lsof \
    iproute2 \
    bash \
    rsync

COPY --from=builder /usr/local/go /usr/local/go
COPY --from=builder /go/bin /usr/local/bin
COPY --from=builder /app /app

WORKDIR /app
ENV GOPATH=/go
ENV PATH=/go/bin:/usr/local/go/bin:/usr/local/bin:$PATH

CMD ["make", "test-all"]