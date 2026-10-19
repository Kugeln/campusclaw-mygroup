FROM campusclaw-week5-rag-test:local
RUN apk add --no-cache mariadb-client mariadb-connector-c
WORKDIR /src
COPY go.mod go.sum ./
ARG GOPROXY=https://proxy.golang.org,direct
RUN GOPROXY=off go mod download
ENV CGO_ENABLED=0 GOCACHE=/tmp/go-build
ENTRYPOINT []
