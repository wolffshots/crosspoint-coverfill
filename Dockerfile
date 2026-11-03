FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
ARG VERSION=dev
ARG TARGETOS TARGETARCH
RUN apk add --no-cache tzdata
WORKDIR /src
COPY go.mod *.go ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /coverfill .

# The final stage has no RUN step, so a multi-arch build needs no emulation.
FROM alpine:3.21
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /coverfill /usr/local/bin/coverfill
COPY --chmod=755 docker/entrypoint.sh /usr/local/bin/entrypoint
COPY --chmod=755 docker/coverfill-all.sh /usr/local/bin/coverfill-all
ENTRYPOINT ["entrypoint"]
