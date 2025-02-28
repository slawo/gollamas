# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG COMMIT_SHA
ARG RELEASE_DATE
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags "-X main.version=$VERSION -X main.buildID=$COMMIT_SHA -X main.buildDate=$DATE" \
    -o gollamas .

FROM alpine:edge
WORKDIR /app
COPY --from=build /app/gollamas .
ENTRYPOINT ["/app/gollamas"]
