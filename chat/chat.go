package chat

import (
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/moby/moby/pkg/namesgenerator"
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
	PriorityHigh  = "primary"
	PriorityLow   = "muted"
	exchangeName  = "chat"
	queuePrefix   = exchangeName + "."
	queueWildcard = queuePrefix + "*"
)

type Message struct {
	Sender   string `json:"sender"`
	Data     string `json:"data"`
	Priority string `json:"priority"`
	Channel  string `json:"channel"`
	Media    string `json:"media"`
}

type Action func(channel *Channel, user *User, data string) error

type Channel struct {
	Name         string
	host         *Server
	participants map[string]*User
}

func (c *Channel) Publish(msg Message) {
	c.host.publish(Message{
		Channel:  c.Name,
		Data:     msg.Data,
		Sender:   msg.Sender,
		Media:    msg.Media,
		Priority: msg.Priority,
	})
}

func (c *Channel) broadcast(msg Message) {
	logrus.WithFields(logrus.Fields{
		"channel": c.Name,
		"sender":  msg.Sender,
		"message": msg.Data,
	}).Info("Broadcasting message to users")
	for _, p := range c.participants {
		p.Send(msg)
	}
}

func (c *Channel) Join(u *User) {
	logrus.WithFields(logrus.Fields{
		"channel": c.Name,
		"user":    u.Name,
	}).Info("User joined channel")
	c.participants[u.Name] = u
	c.Publish(Message{
		Sender:   c.host.Name,
		Data:     u.Name + " joined the channel",
		Channel:  c.Name,
		Priority: PriorityLow,
	})
}

func (c *Channel) Leave(u *User) {
	logrus.WithFields(logrus.Fields{
		"channel": c.Name,
		"user":    u.Name,
	}).Info("User left channel")
	delete(c.participants, u.Name)
	c.Publish(Message{
		Sender:   c.host.Name,
		Data:     u.Name + " left the channel",
		Channel:  c.Name,
		Priority: PriorityLow,
	})
}

type User struct {
	Name   string
	conn   *websocket.Conn
	active *Channel
	host   *Server
}

func (user *User) Watch() {
	logrus.WithFields(logrus.Fields{
		"user": user.Name,
	}).Info("Watching user input")
	var text string
	var lastMessage time.Time
	for {
		if err := websocket.Message.Receive(user.conn, &text); err != nil {
			break
		}
		if interval := time.Since(lastMessage); interval < user.host.textInterval {
			continue
		}
		lastMessage = time.Now()
		if len(text) > user.host.textLimit {
			continue
		}
		logrus.WithFields(logrus.Fields{
			"message": text,
			"user":    user.Name,
		}).Info("Received message from user")
		text = strings.TrimSpace(text)
		if len(text) < 1 {
			continue
		}
		command := strings.SplitN(text, " ", 2)
		if action, ok := user.host.actions[command[0]]; ok {
			err := action(user.active, user, command[len(command)-1])
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"user":    user.Name,
					"channel": user.active.Name,
					"action":  command[0],
					"error":   err,
				}).Warn("Failed to invoke action")
			}
			continue
		}
		user.active.Publish(Message{
			Sender:  user.Name,
			Data:    text,
			Channel: user.active.Name,
		})
	}
	user.active.Leave(user)
	logrus.WithFields(logrus.Fields{
		"user": user.Name,
	}).Info("Closing connection")
}

func (user *User) Send(msg Message) error {
	return websocket.JSON.Send(user.conn, msg)
}

func Capitalize(s ...string) []string {
	if len(s) == 1 {
		p := s[0]
		if len(p) > 1 {
			return []string{strings.ToUpper(p[:1]) + p[1:]}
		}
		return []string{strings.ToUpper(p)}
	}
	for i, p := range s {
		s[i] = Capitalize(p)[0]
	}
	return s
}

func NewUser(conn *websocket.Conn, host *Server) *User {
	name := namesgenerator.GetRandomName(0)
	return &User{
		Name: strings.Join(Capitalize(strings.Split(name, "_")...), " "),
		conn: conn,
		host: host,
	}
}

func NewChannel(name string, host *Server) *Channel {
	return &Channel{
		Name:         name,
		participants: map[string]*User{},
		host:         host,
	}
}

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

func (s *Server) AddChannel(channel *Channel) {
	s.channels[channel.Name] = channel
}

func (s *Server) AddAction(tag string, action Action) {
	s.actions[tag] = action
}

func (s *Server) Accept(conn *websocket.Conn) {
	logrus.WithFields(logrus.Fields{
		"remote": conn.RemoteAddr(),
		"local":  conn.LocalAddr(),
	}).Info("Connected with client")

	user := NewUser(conn, s)
	defer user.Watch()
	logrus.WithFields(logrus.Fields{
		"user": user.Name,
	}).Info("Generated new user")

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
		}).Warn("Failed to open channel")
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

func WithAction(tag string, action Action) Option {
	return func(s *Server) {
		s.actions[tag] = action
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
