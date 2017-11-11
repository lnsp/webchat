package blueprint

import (
	"time"

	"github.com/lnsp/webchat/chat"
)

func PrivateResponse(senderName, channelName, data, media string) chat.Handler {
	return func(server *chat.Server, channel *chat.Channel, user *chat.User, command string) error {
		return user.Send(chat.Message{
			Sender:  senderName,
			Data:    data,
			Media:   media,
			Channel: channelName,
		})
	}
}

func BroadcastResponse(data, media string) chat.Handler {
	return func(server *chat.Server, channel *chat.Channel, user *chat.User, command string) error {
		channel.Publish(chat.Message{
			Sender: user.Name,
			Data:   data,
			Media:  media,
		})
		return nil
	}
}

func RateLimitMiddleware(invoke chat.Handler, interval time.Duration, message string) chat.Handler {
	var timer time.Time
	return func(server *chat.Server, channel *chat.Channel, user *chat.User, command string) error {
		if time.Since(timer) < interval {
			user.Send(chat.Message{
				Priority: chat.PriorityLow,
				Channel:  channel.Name,
				Data:     message,
			})
			return nil
		}
		timer = time.Now()
		return invoke(server, channel, user, command)
	}
}
