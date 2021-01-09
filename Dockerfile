FROM golang:1.14.4 as builder
WORKDIR /go/src

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o=kubetower -ldflags='-w -extldflags "-static"'

FROM scratch

COPY --from=builder /go/src/kubetower .
EXPOSE 8080

ENTRYPOINT ["/kubetower"]