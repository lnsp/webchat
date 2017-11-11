# webchat

*webchat* is a single-instance chat service without persistency. The chat history is not stored or cached on the server. *webchat* functions as a simple relay that includes configurable actions between the clients. It uses WebSockets to communicate fast and securely.

It can be configured using a simple *config.yaml* file, an example can be found in the repository.

## Installation
```
$ go get github.com/lnsp/webchat
$ PORT=8080 $GOPATH/bin/webchat
...
```