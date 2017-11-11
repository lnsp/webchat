package chat

import (
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/streadway/amqp"
	"golang.org/x/net/websocket"
)

const (
	defaultServerName   = "WebChat"
	defaultChannelName  = "default"
	defaultMOTD         = "Welcome to WebChat!"
	defaultTextInterval = 10 * time.Millisecond
	defaultTextLimit    = 140
)

const (
	exchangeName  = "chat"
	queuePrefix   = exchangeName + "."
	queueWildcard = queuePrefix + "*"
)

type Server struct {
	Name         string
	motd         string
	channels     map[string]*Channel
	textInterval time.Duration
	textLimit    int
	actions      map[string]Action
	broker       *amqp.Connection
	mainChannel  string
}

func (s *Server) ListActions() []Action {
	actions := make([]Action, 0, len(s.actions))
	for _, a := range s.actions {
		actions = append(actions, a)
	}
	return actions
}

func (s *Server) List() []*User {
	users := make(map[*User]bool)
	for _, c := range s.channels {
		for _, u := range c.List() {
			users[u] = true
		}
	}
	endUsers := make([]*User, 0, len(users))
	for u := range users {
		endUsers = append(endUsers, u)
	}
	return endUsers
}

func (s *Server) ListChannels() []*Channel {
	channels := make([]*Channel, 0, len(s.channels))
	for _, c := range s.channels {
		channels = append(channels, c)
	}
	return channels
}

func (s *Server) AddChannel(channel *Channel) {
	s.channels[channel.Name] = channel
}

func (s *Server) AddAction(action Action) {
	logrus.WithFields(logrus.Fields{
		"name":        action.Name,
		"description": action.Description,
	}).Debug("Add action to server")
	s.actions["!"+action.Name] = action
}

func (s *Server) Accept(conn *websocket.Conn) {
	logrus.WithFields(logrus.Fields{
		"remote": conn.RemoteAddr(),
		"local":  conn.LocalAddr(),
	}).Debug("Connected with client")

	user := NewUser(conn, s)
	defer user.Watch()
	logrus.WithFields(logrus.Fields{
		"user": user.Name,
	}).Debug("Generated new user")

	user.Send(Message{
		Sender: s.Name,
		Data:   s.motd,
	})
	if len(s.channels) < 1 {
		s.AddChannel(NewChannel(defaultChannelName, s))
		s.mainChannel = defaultChannelName
	}
	main := s.channels[s.mainChannel]
	user.active = main
	main.Join(user)
}

func (s *Server) Handler() http.Handler {
	if s.broker == nil {
		logrus.Fatal("Not connected to message queue")
	}
	return websocket.Handler(s.Accept)
}

func (s *Server) publish(msg Message) {
	ch, err := s.broker.Channel()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"broker": s.broker.LocalAddr(),
		}).Debug("Failed to open channel")
		return
	}
	defer ch.Close()
	bytes, err := json.Marshal(msg)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"channel": msg.Channel,
			"sender":  msg.Sender,
		}).Warn("Could not marshal message")
		return
	}
	if err := ch.Publish(exchangeName, queueWildcard, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        bytes,
	}); err != nil {
		logrus.WithFields(logrus.Fields{
			"broker": s.broker.LocalAddr(),
			"queue":  queuePrefix + "*",
		}).Warn("Could not publish message")
	}
}

func (s *Server) route(msg Message) {
	channel, ok := s.channels[msg.Channel]
	if !ok {
		logrus.WithFields(logrus.Fields{
			"channel": msg.Channel,
			"sender":  msg.Sender,
		}).Warn("Could not route message")
		return
	}
	channel.broadcast(msg)
}

func (s *Server) consumeLoop() {
	host, err := os.Hostname()
	if err != nil {
		// generate random bytes instead
		rand.Seed(time.Now().Unix())
		var randBytes [8]byte
		rand.Read(randBytes[:])
		host = hex.EncodeToString(randBytes[:])
	}

	ch, err := s.broker.Channel()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err":    err,
			"broker": s.broker.LocalAddr(),
		}).Fatal("Could not open channel")
	}

	if err := ch.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		logrus.WithFields(logrus.Fields{
			"exchange": exchangeName,
			"err":      err,
		}).Fatal("Could not declare exchange")
	}
	logrus.WithFields(logrus.Fields{
		"exchange": exchangeName,
	}).Info("Declared chat exchange")

	queue, err := ch.QueueDeclare(queuePrefix+host, false, false, false, false, nil)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err":    err,
			"broker": s.broker.LocalAddr(),
			"name":   host,
		}).Fatal("Could not declare queue")
	}
	logrus.WithFields(logrus.Fields{
		"queue": queue.Name,
	}).Info("Declared public queue")

	if err := ch.QueueBind(queue.Name, queueWildcard, exchangeName, false, nil); err != nil {
		logrus.WithFields(logrus.Fields{
			"err":    err,
			"broker": s.broker.LocalAddr(),
			"name":   host,
		}).Fatal("Could not bind queue to exchange")
	}

	for {
		incoming, err := ch.Consume(queue.Name, "", true, false, false, false, nil)
		if err != nil {
			ch.Close()
			ch, err = s.broker.Channel()
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"err":    err,
					"broker": s.broker.LocalAddr(),
				}).Fatal("Could not open channel")
			}
		}
		for payload := range incoming {
			var msg Message
			if err := json.Unmarshal(payload.Body, &msg); err != nil {
				logrus.WithFields(logrus.Fields{
					"id":    payload.MessageId,
					"queue": queue.Name,
				}).Warn("Failed to consume message")
				continue
			}
			go s.route(msg)
		}
	}
}

func (s *Server) Connect(broker string) error {
	conn, err := amqp.Dial(broker)
	if err != nil {
		return errors.Wrap(err, "Could not init server")
	}
	logrus.WithFields(logrus.Fields{
		"broker": broker,
	}).Info("Connected to message queue")
	s.broker = conn
	go s.consumeLoop()
	return nil
}

func New(options ...Option) *Server {
	rand.Seed(time.Now().Unix())
	server := &Server{
		Name:         defaultServerName,
		motd:         defaultMOTD,
		channels:     map[string]*Channel{},
		textInterval: defaultTextInterval,
		textLimit:    defaultTextLimit,
		actions:      map[string]Action{},
	}
	for _, opt := range options {
		opt(server)
	}
	for _, act := range DefaultActions {
		server.AddAction(act)
	}
	return server
}

type Option func(s *Server)

func WithName(name string) Option {
	return func(s *Server) {
		s.Name = name
	}
}

func WithChannels(names ...string) Option {
	return func(s *Server) {
		for _, n := range names {
			s.AddChannel(NewChannel(n, s))
		}
	}
}

func WithMOTD(motd string) Option {
	return func(s *Server) {
		s.motd = motd
	}
}

func WithAction(action Action) Option {
	return func(s *Server) {
		s.AddAction(action)
	}
}

func WithTextLimit(limit int) Option {
	return func(s *Server) {
		s.textLimit = limit
	}
}

func WithTextInterval(interval time.Duration) Option {
	return func(s *Server) {
		s.textInterval = interval
	}
}

func WithMainChannel(name string) Option {
	return func(s *Server) {
		s.mainChannel = name
	}
}
