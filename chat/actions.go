package chat

import (
	"strings"
)

var (
	DefaultActions = []Action{
		{
			Name:        "help",
			Description: "Show help information",
			Invoke:      showHelp,
		},
		{
			Name:        "users",
			Description: "List users in channel",
			Invoke:      listUsers,
		},
		{
			Name:        "channels",
			Description: "List channels on server",
			Invoke:      listChannels,
		},
	}
)

func showHelp(host *Server, channel *Channel, user *User, command string) error {
	actions := host.ListActions()
	actionNames := make([]string, len(actions))
	for i, a := range actions {
		actionNames[i] = "!" + a.Name
	}
	message := "Available actions are " + strings.Join(actionNames, ", ") + "."
	user.Send(Message{
		Priority: PriorityLow,
		Data:     message,
	})
	return nil
}

func listUsers(host *Server, channel *Channel, user *User, command string) error {
	return nil
}

func listChannels(host *Server, channel *Channel, user *User, command string) error {
	return nil
}
