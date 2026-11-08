# moki with Chromium, so screenshots work.
#
#   docker run -d --name moki -p 9999:9999 -v moki:/data -e TZ=Asia/Kolkata ghcr.io/tanmoysrt/moki
#   docker exec moki moki token register
#
# MOKI_URL is the URL that agents use. Change it when you publish another port.

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /moki .

FROM alpine:3.22
RUN apk add --no-cache chromium font-noto font-noto-emoji tzdata \
 && adduser -D moki \
 && mkdir /data \
 && chown moki /data
COPY --from=build /moki /usr/local/bin/moki
USER moki
ENV MOKI_HOST=0.0.0.0 \
    MOKI_HOME=/data \
    MOKI_URL=http://localhost:9999 \
    CHROME=/usr/bin/chromium
VOLUME /data
EXPOSE 9999
HEALTHCHECK CMD wget -qO /dev/null http://127.0.0.1:9999/health || exit 1
ENTRYPOINT ["moki"]
