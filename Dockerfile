# syntax=docker/dockerfile:1
# Builds any of this module's cmd/ binaries, selected via CMD_PATH build arg.
# Used for the main service (cmd/server) and the local-only mocks
# (cmd/mockpbxworker, cmd/mocklaravel) — see docker-compose.yml.

FROM golang:1.22-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG CMD_PATH=cmd/server
RUN CGO_ENABLED=0 go build -o /out/app ./${CMD_PATH}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/app /app
ENTRYPOINT ["/app"]
