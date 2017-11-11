package chat

import (
	"github.com/Sirupsen/logrus"
)

type Action func(channel *Channel, user *User, data string) error

type Channel struct {
	Name         string
	host         *Server
	participants map[string]*User
}

func (c *Channel) List() []*User {
	users := make([]*User, 0, len(c.participants))
	for _, u := range c.participants {
		users = append(users, u)
	}
	return users
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

func NewChannel(name string, host *Server) *Channel {
	return &Channel{
		Name:         name,
		participants: map[string]*User{},
		host:         host,
	}
}
