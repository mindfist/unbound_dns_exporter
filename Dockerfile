FROM golang:latest

WORKDIR /app
RUN go get -d ./...
RUN CGO_ENABLED=0 GOOS=linux go build --ldflags '-extldflags "-static"' -o unbound_exporter
RUN strip unbound_dns_exporter

FROM alpine:latest
WORKDIR /root
COPY --from=0 /app/unbound_dns_exporter .

EXPOSE 9167
CMD ["./unbound_exporter"]