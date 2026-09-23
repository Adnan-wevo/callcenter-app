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
# tzdata is required for internal/reports.Location() to actually resolve
# "Asia/Kuala_Lumpur" — without it, time.LoadLocation fails (no
# /usr/share/zoneinfo in this base image at all) and Location() silently
# falls back to UTC, which shifts every report's default "today" window 8
# hours earlier than real KL time. That's not a hypothetical: it's why a
# call placed this afternoon (KL time) didn't show up in the Dashboard
# under the DEFAULT date filter — an explicit date_from/date_to found it
# fine, only the UTC-shifted default window missed it.
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/app /app
ENTRYPOINT ["/app"]
