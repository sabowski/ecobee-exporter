FROM golang:1.21-alpine AS builder
WORKDIR /src
COPY . .
RUN go mod download
RUN go build -o build/ecobee-exporter ./main.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /src/build/ecobee-exporter /bin
EXPOSE 9500
USER nonroot:nonroot
ENTRYPOINT ["/bin/ecobee-exporter"]
