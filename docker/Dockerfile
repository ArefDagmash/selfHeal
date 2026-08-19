# ---- Go service (multi-stage) ----
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/testforge ./cmd/testforge

# alpine is light and chromium is available via apk; the Go binary is static
# (CGO_ENABLED=0) so it runs fine on musl.
FROM alpine:3.20
RUN apk add --no-cache chromium
ENV CHROME_PATH=/usr/bin/chromium
WORKDIR /data
VOLUME /data
EXPOSE 8080
COPY --from=build /out/testforge /usr/local/bin/testforge
CMD ["testforge"]
