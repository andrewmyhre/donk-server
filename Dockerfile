FROM golang:1.14 as builder

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
COPY ./ ./

ENV GO111MODULE=on

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o ./bin/donk-server main.go

FROM scratch

WORKDIR /
COPY --from=builder /workspace/bin/donk-server .
ENTRYPOINT ["/donk-server"]