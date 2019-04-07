# Start with a full-fledged golang image, but strip it from the final image.
FROM golang:1.12.1-alpine

# That's me!
LABEL maintainer="Gary Gordon <gagordon12@gmail.com>"

WORKDIR /go/src/github.com/gdotgordon/produce-demo

COPY . /go/src/github.com/gdotgordon/produce-demo

RUN go build -v

FROM alpine:latest

WORKDIR /root/

# Make a significantly slimmed-down final result.
COPY --from=0 /go/src/github.com/gdotgordon/produce-demo .

ENTRYPOINT ["./produce-demo"]
CMD ["--port=8080"]
