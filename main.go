package main

import (
	"net/http"
	"os"

	logrus "github.com/Sirupsen/logrus"
	"github.com/moby/moby/pkg/namesgenerator"
	"golang.org/x/net/websocket"
)

type Channel struct {
	Participants map[string]*User
	Name         string
}

func (c *Channel) Broadcast(msg Message) {
	logrus.WithFields(logrus.Fields{
		"channel": c.Name,
		"sender":  msg.Sender,
		"message": msg.Text,
	}).Info("Broadcasting message to users")
	for _, p := range c.Participants {
		p.Send(msg)
	}
}

func (c *Channel) Join(u *User) {
	logrus.WithFields(logrus.Fields{
		"channel": c.Name,
		"user":    u.Name,
	}).Info("User joined channel")
	c.Participants[u.Name] = u
	c.Broadcast(Message{
		Sender:  "server",
		Text:    u.Name + " joined the channel",
		Channel: c.Name,
	})
}

func (c *Channel) Leave(u *User) {
	logrus.WithFields(logrus.Fields{
		"channel": c.Name,
		"user":    u.Name,
	}).Info("User left channel")
	delete(c.Participants, u.Name)
	c.Broadcast(Message{
		Sender:  "server",
		Text:    u.Name + " left the channel",
		Channel: c.Name,
	})
}

type Message struct {
	Sender  string `json:"sender"`
	Text    string `json:"text"`
	Channel string `json:"channel"`
}

type User struct {
	Conn      *websocket.Conn
	Name      string
	SendingTo *Channel
}

func (p *User) Watch() {
	logrus.WithField("name", p.Name).Info("Watching user input")
	var text string
	for {
		if err := websocket.Message.Receive(p.Conn, &text); err != nil {
			break
		}
		logrus.WithFields(logrus.Fields{
			"message": text,
			"name":    p.Name,
		}).Info("Received message from user")
		msg := Message{
			Sender:  p.Name,
			Text:    text,
			Channel: p.SendingTo.Name,
		}
		p.SendingTo.Broadcast(msg)
	}
	p.SendingTo.Leave(p)
	logrus.WithField("name", p.Name).Info("Closing connection")
}

func (p *User) Send(msg Message) error {
	return websocket.JSON.Send(p.Conn, msg)
}

func NewUser(conn *websocket.Conn) *User {
	return &User{
		Name: namesgenerator.GetRandomName(0),
		Conn: conn,
	}
}

func NewChannel(name string) *Channel {
	return &Channel{
		Name:         name,
		Participants: map[string]*User{},
	}
}

func main() {
	mainChannel := NewChannel("main")
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.Handle("/chat/", websocket.Handler(func(conn *websocket.Conn) {
		logrus.WithField("remote", conn.RemoteAddr()).Info("Connected with client")
		user := NewUser(conn)
		logrus.WithField("name", user.Name).Info("Generated new user")
		defer user.Watch()

		user.SendingTo = mainChannel
		mainChannel.Join(user)
	}))

	http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
