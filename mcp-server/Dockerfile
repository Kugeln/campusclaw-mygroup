FROM golang:1.26.0-alpine3.23 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
ENV CGO_ENABLED=0
RUN go build -trimpath -o /out/mcp-server ./cmd/mcp-server && go build -trimpath -o /out/probe ./cmd/probe

FROM scratch AS server
COPY --from=build /out/mcp-server /mcp-server
USER 10001:10001
EXPOSE 8090
HEALTHCHECK --interval=2s --timeout=3s --retries=15 CMD ["/mcp-server", "healthcheck"]
ENTRYPOINT ["/mcp-server"]

FROM scratch AS probe
COPY --from=build /out/probe /probe
USER 10001:10001
ENTRYPOINT ["/probe"]
