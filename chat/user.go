package chat

import (
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/moby/moby/pkg/namesgenerator"
	"golang.org/x/net/websocket"
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
