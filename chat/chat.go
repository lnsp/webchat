package chat

import (
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/moby/moby/pkg/namesgenerator"
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
	PriorityHigh = "primary"
	PriorityLow  = "muted"
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

func (c *Channel) Broadcast(msg Message) {
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
	c.Broadcast(Message{
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
	c.Broadcast(Message{
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
		user.active.Broadcast(Message{
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
	channels     []*Channel
	textInterval time.Duration
	textLimit    int
	actions      map[string]Action
}

func (s *Server) AddChannel(channel *Channel) {
	s.channels = append(s.channels, channel)
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
	}
	main := s.channels[0]
	user.active = main
	main.Join(user)
}

func (s *Server) Handler() http.Handler {
	return websocket.Handler(s.Accept)
}

func New(options ...Option) *Server {
	server := &Server{
		Name:         defaultServerName,
		motd:         defaultMOTD,
		channels:     []*Channel{},
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
		s.channels = make([]*Channel, len(names))
		for i, n := range names {
			s.channels[i] = NewChannel(n, s)
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
