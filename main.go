package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	logrus "github.com/Sirupsen/logrus"
	"github.com/moby/moby/pkg/namesgenerator"
	"golang.org/x/net/websocket"
)

const timInterval = 50 * time.Millisecond
const maxCharLimit = 120

type Action func(channel *Channel, user *User, data string)

func actionTumlerTaxi(channel *Channel, user *User, data string) {
	channel.Broadcast(Message{
		Sender:  "server",
		Text:    "https://user-images.githubusercontent.com/3391295/32673053-cd92abf6-c64d-11e7-9172-e11a9c3c5343.jpg",
		Media:   "image",
		Channel: "TumBWL",
	})
}

func actionShowMe(channel *Channel, user *User, data string) {
	channel.Broadcast(Message{
		Sender:  "server",
		Text:    "https://media.giphy.com/media/26DOs997h6fgsCthu/giphy.gif",
		Media:   "image",
		Channel: channel.Name,
	})
}

func actionLeberkas(channel *Channel, user *User, data string) {
	channel.Broadcast(Message{
		Sender:  "server",
		Text:    "http://www.wilding.at/img/products/leber3.jpg",
		Media:   "image",
		Channel: channel.Name,
	})
}

type Channel struct {
	Actions      map[string]Action
	Participants map[string]*User
	Name         string
}

func (c *Channel) Add(command string, action Action) {
	c.Actions[command] = action
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
	Media   string `json:"media"`
}

type User struct {
	Conn      *websocket.Conn
	Name      string
	SendingTo *Channel
}

func (user *User) Watch() {
	logrus.WithField("name", user.Name).Info("Watching user input")
	var text string
	var lastMessage time.Time
	for {
		if err := websocket.Message.Receive(user.Conn, &text); err != nil {
			break
		}
		// filter out spam
		if interval := time.Since(lastMessage); interval < timInterval {
			continue
		}
		lastMessage = time.Now()
		// filter our too long messages
		if len(text) > maxCharLimit {
			continue
		}
		logrus.WithFields(logrus.Fields{
			"message": text,
			"name":    user.Name,
		}).Info("Received message from user")
		// filter out short messages
		text = strings.TrimSpace(text)
		if len(text) < 1 {
			continue
		}
		command := strings.SplitN(text, " ", 2)
		if action, ok := user.SendingTo.Actions[command[0]]; ok {
			action(user.SendingTo, user, command[len(command)-1])
			continue
		}
		user.SendingTo.Broadcast(Message{
			Sender:  user.Name,
			Text:    text,
			Channel: user.SendingTo.Name,
		})
	}
	user.SendingTo.Leave(user)
	logrus.WithField("name", user.Name).Info("Closing connection")
}

func (user *User) Send(msg Message) error {
	return websocket.JSON.Send(user.Conn, msg)
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
		Actions:      map[string]Action{},
	}
}

func main() {
	mainChannel := NewChannel("main")
	mainChannel.Add("!vollgas", actionLeberkas)
	mainChannel.Add("!show", actionShowMe)
	mainChannel.Add("!tumbwl", actionTumlerTaxi)

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
