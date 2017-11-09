package main

import (
	"net/http"
	"os"

	"github.com/moby/moby/pkg/namesgenerator"
	"golang.org/x/net/websocket"
)

type Channel struct {
	Participants []User
	Name         string
}

func (c Channel) Broadcast(msg Message) {
	for _, p := range c.Participants {
		p.Send(msg)
	}
}

type Message struct {
	Sender  string `json:"sender"`
	Text    string `json:"text"`
	Channel string `json:"channel"`
}

type User struct {
	Conn *websocket.Conn
	Name string
}

func (p User) Join(channel *Channel) {
	channel.Participants = append(channel.Participants, p)
	channel.Broadcast(Message{
		Sender: "server",
		Text:   "User " + p.Name + " joined the server.",
	})
	var text string
	for {
		if err := websocket.Message.Receive(p.Conn, &text); err != nil {
			break
		}
		msg := Message{
			Sender:  p.Name,
			Text:    text,
			Channel: channel.Name,
		}
		channel.Broadcast(msg)
	}
}

func (p User) Send(msg Message) error {
	return websocket.JSON.Send(p.Conn, msg)
}

func NewUser(conn *websocket.Conn) User {
	return User{
		Name: namesgenerator.GetRandomName(0),
		Conn: conn,
	}
}

func NewChannel(name string) *Channel {
	return &Channel{
		Name:         name,
		Participants: []User{},
	}
}

func main() {
	defaultChan := NewChannel("default")
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.Handle("/chat/", websocket.Handler(func(conn *websocket.Conn) {
		user := NewUser(conn)
		user.Join(defaultChan)
	}))

	http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
