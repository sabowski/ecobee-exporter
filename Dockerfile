FROM golang:1.22.4-alpine AS builder
WORKDIR /src
COPY . .
RUN go mod download
# hadolint ignore=DL3059
RUN go build -o build/ecobee-exporter ./main.go

FROM alpine:3.19.1
WORKDIR /
COPY --from=builder /src/build/ecobee-exporter /bin
EXPOSE 9500
USER nobody:nogroup
ENTRYPOINT ["/bin/ecobee-exporter"]
