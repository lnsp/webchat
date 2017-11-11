package action

import (
	"time"

	"github.com/lnsp/webchat/chat"
)

func PrivateResponse(data, media string) chat.Action {
	return func(channel *chat.Channel, user *chat.User, command string) error {
		return user.Send(chat.Message{
			Sender:  channel.Name,
			Data:    data,
			Media:   media,
			Channel: channel.Name,
		})
	}
}

func BroadcastResponse(data, media string) chat.Action {
	return func(channel *chat.Channel, user *chat.User, command string) error {
		channel.Broadcast(chat.Message{
			Sender:  user.Name,
			Data:    data,
			Media:   media,
			Channel: channel.Name,
		})
		return nil
	}
}

func RateLimitMiddleware(invoke chat.Action, interval time.Duration, message string) chat.Action {
	var timer time.Time
	return func(channel *chat.Channel, user *chat.User, command string) error {
		if time.Since(timer) < interval {
			user.Send(chat.Message{
				Priority: chat.PriorityLow,
				Channel:  channel.Name,
				Data:     message,
			})
			return nil
		}
		timer = time.Now()
		return invoke(channel, user, command)
	}
}
