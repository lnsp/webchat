# webchat [![Build Status](https://travis-ci.org/lnsp/webchat.svg?branch=master)](https://travis-ci.org/lnsp/webchat)

*webchat* is a scalable chat service without persistency. It uses RabbitMQ to communicate between each frontend instance. The chat history is neither stored nor cached on the server. *webchat* functions as a simple relay that includes configurable actions between the clients. It uses WebSockets to communicate fast and securely.

It can be configured using a simple *config.yaml* file, an example can be found in the repository.

## Installation
```
$ go get github.com/lnsp/webchat
$ PORT=8080 $GOPATH/bin/webchat
...
```