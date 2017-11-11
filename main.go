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
	if os.Getenv("DEBUG") != "" {
		logrus.SetLevel(logrus.DebugLevel)
	}
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
	if err := server.Connect(os.Getenv("RABBITMQ_URL")); err != nil {
		logrus.WithFields(logrus.Fields{
			"url": os.Getenv("RABBITMQ_URL"),
			"err": err,
		}).Fatal("Could not connect to message broker")
	}
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.Handle("/chat/", server.Handler())
	http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
