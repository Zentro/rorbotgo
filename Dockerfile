FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETARCH=amd64
RUN make build && cp build/rorbot_linux_${TARGETARCH} rorbotgo

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S rorbot && adduser -S -G rorbot rorbot

RUN mkdir -p /etc/rorbotgo /var/lib/rorbotgo /var/log/rorbotgo \
    && chown -R rorbot:rorbot /var/lib/rorbotgo /var/log/rorbotgo

COPY --from=builder /build/rorbotgo /usr/local/bin/rorbotgo

USER rorbot

ENTRYPOINT ["rorbotgo"]
