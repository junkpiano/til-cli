FROM golang:1.18-alpine

COPY . /app
WORKDIR /app

RUN go build

ENTRYPOINT ["/app/entrypoint.sh"]
