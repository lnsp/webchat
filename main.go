package main

import (
	"net/http"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/lnsp/webchat/chat"
	"github.com/lnsp/webchat/config"
)

func main() {
	var (
		server *chat.Server
		err    error
	)
	if cfg := os.Getenv("CONFIG_FILE"); cfg != "" {
		server, err = config.Build(cfg)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"config": cfg,
				"error":  err,
			}).Fatal("Could not build server from configuration")
		}
		logrus.WithField("config", cfg).Info("Using configuration file")
	} else {
		server = chat.New()
		logrus.Info("Using default configuration")
	}
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.Handle("/chat/", server.Handler())
	http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
