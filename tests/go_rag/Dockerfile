FROM golang:1.26.0-alpine3.23
WORKDIR /src
COPY go.mod go.sum ./
ARG GOPROXY=https://proxy.golang.org,direct
RUN GOPROXY=$GOPROXY go mod download
ENV GOCACHE=/tmp/go-cache
ENTRYPOINT ["go", "test"]
