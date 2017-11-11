FROM golang:1.9

RUN mkdir -p /go/src/github.com/lnsp/webchat
WORKDIR /go/src/github.com/lnsp/webchat
COPY . .

RUN go-wrapper install

CMD ["go-wrapper", "run"]
