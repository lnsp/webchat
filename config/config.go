package config

import (
	"io/ioutil"
	"strconv"
	"time"

	"github.com/lnsp/webchat/chat"
	"github.com/lnsp/webchat/chat/blueprint"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

type Middleware map[string]string

type Action struct {
	Tag         string                `yaml:"tag"`
	Description string                `yaml:"description"`
	Type        string                `yaml:"type"`
	Media       string                `yaml:"media"`
	Data        string                `yaml:"data"`
	Sender      string                `yaml:"sender"`
	Channel     string                `yaml:"channel"`
	Middleware  map[string]Middleware `yaml:"middleware"`
}

type Chat struct {
	Actions  []Action `yaml:"actions"`
	Channels []string `yaml:"channels,flow"`
	General  struct {
		Name            string `yaml:"name"`
		MOTD            string `yaml:"motd"`
		CharacterLimit  int    `yaml:"characterLimit"`
		MessageInterval int    `yaml:"messageInterval"`
		MainChannel     string `yaml:"mainChannel"`
	}
}

func Build(file string) (*chat.Server, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "could not read configuration")
	}
	var config Chat
	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return nil, errors.Wrap(err, "could not read configuration")
	}
	server := chat.New(
		chat.WithName(config.General.Name),
		chat.WithMOTD(config.General.MOTD),
		chat.WithMainChannel(config.General.MainChannel),
		chat.WithChannels(config.Channels...),
		chat.WithTextLimit(config.General.CharacterLimit),
		chat.WithTextInterval(time.Duration(config.General.MessageInterval)*time.Millisecond),
	)
	for _, act := range config.Actions {
		var generated chat.Handler
		switch act.Type {
		case "private":
			generated = blueprint.PrivateResponse(act.Sender, act.Channel, act.Data, act.Media)
		case "broadcast":
			generated = blueprint.BroadcastResponse(act.Data, act.Media)
		default:
			return nil, errors.Errorf("unknown action type %s", act.Type)
		}
		for name, middleware := range act.Middleware {
			switch name {
			case "limit":
				interval, err := strconv.Atoi(middleware["interval"])
				if err != nil {
					return nil, errors.Wrap(err, "could not read rate limit")
				}
				generated = blueprint.RateLimitMiddleware(generated, time.Duration(interval)*time.Second, middleware["message"])
			default:
				return nil, errors.Errorf("unknown middleware type %s", name)
			}
		}
		server.AddAction(chat.NewAction(act.Tag, act.Description, generated))
	}
	return server, nil
}
