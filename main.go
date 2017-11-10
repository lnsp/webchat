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

const priorityLow = "muted"
const serverName = "WebChat"
const timInterval = 50 * time.Millisecond
const maxCharLimit = 120

type Action func(channel *Channel, user *User, data string)

func actionTumlerTaxi(interval time.Duration) Action {
	var lastLeberkas time.Time
	return func(channel *Channel, user *User, data string) {
		if time.Since(lastLeberkas) < interval {
			user.Send(Message{
				Sender:  serverName,
				Data:    "Your last Leberkas! Leben am Limit.",
				Channel: channel.Name,
			})
		} else {
			lastLeberkas = time.Now()
			channel.Broadcast(Message{
				Sender:  user.Name,
				Data:    "https://user-images.githubusercontent.com/3391295/32673053-cd92abf6-c64d-11e7-9172-e11a9c3c5343.jpg",
				Media:   "image",
				Channel: channel.Name,
			})
		}
	}
}

func actionShowMe(channel *Channel, user *User, data string) {
	channel.Broadcast(Message{
		Sender:  user.Name,
		Data:    "https://media.giphy.com/media/26DOs997h6fgsCthu/giphy.gif",
		Media:   "image",
		Channel: channel.Name,
	})
}

func actionLeberkas(channel *Channel, user *User, data string) {
	channel.Broadcast(Message{
		Sender:  user.Name,
		Data:    "http://www.wilding.at/img/products/leber3.jpg",
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
		"message": msg.Data,
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
		Sender:   serverName,
		Data:     u.Name + " joined the channel",
		Channel:  c.Name,
		Priority: priorityLow,
	})
}

func (c *Channel) Leave(u *User) {
	logrus.WithFields(logrus.Fields{
		"channel": c.Name,
		"user":    u.Name,
	}).Info("User left channel")
	delete(c.Participants, u.Name)
	c.Broadcast(Message{
		Sender:   serverName,
		Data:     u.Name + " left the channel",
		Channel:  c.Name,
		Priority: priorityLow,
	})
}

type Message struct {
	Sender   string `json:"sender"`
	Data     string `json:"data"`
	Priority string `json:"priority"`
	Channel  string `json:"channel"`
	Media    string `json:"media"`
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
			Data:    text,
			Channel: user.SendingTo.Name,
		})
	}
	user.SendingTo.Leave(user)
	logrus.WithField("name", user.Name).Info("Closing connection")
}

func (user *User) Send(msg Message) error {
	return websocket.JSON.Send(user.Conn, msg)
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

func NewUser(conn *websocket.Conn) *User {
	name := namesgenerator.GetRandomName(0)
	return &User{
		Name: strings.Join(Capitalize(strings.Split(name, "_")...), " "),
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
	mainChannel.Add("!tumbwl", actionTumlerTaxi(time.Minute))

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
