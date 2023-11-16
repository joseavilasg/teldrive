package tgc

import (
	"sync"

	"github.com/gotd/contrib/bg"
	"github.com/gotd/td/telegram"
)

type BotWorkers struct {
	sync.Mutex
	bots      []string
	index     int
	channelId int64
}

func (w *BotWorkers) Set(bots []string, channelId *int64) {
	w.Lock()
	defer w.Unlock()
	if channelId != nil && *channelId != w.channelId {
		w.bots = bots
		w.channelId = *channelId
	} else if len(w.bots) == 0 {
		w.bots = bots
	}
}

func (w *BotWorkers) Next() string {
	w.Lock()
	defer w.Unlock()
	w.index = (w.index + 1) % len(w.bots)
	item := w.bots[w.index]
	return item
}

var Workers = &BotWorkers{}

type Client struct {
	Tg     *telegram.Client
	Stop   bg.StopFunc
	Status string
}

type streamWorkers struct {
	sync.Mutex
	bots      []string
	channelId int64
	clients   []*Client
	index     int
}

func (w *streamWorkers) Set(bots []string, channelId *int64) {
	w.Lock()
	defer w.Unlock()

	setupClients := func(replace bool) {
		for _, token := range bots {
			client, _ := BotLogin(token)
			if replace {
				w.clients = []*Client{}
			}
			w.clients = append(w.clients, &Client{Tg: client, Status: "idle"})
		}
	}

	if channelId != nil && *channelId != w.channelId {
		w.bots = bots
		w.channelId = *channelId
		setupClients(true)
	} else if len(w.clients) == 0 {
		w.bots = bots
		setupClients(false)
	}
}

func (w *streamWorkers) Next() (*Client, int, error) {
	w.Lock()
	defer w.Unlock()
	w.index = (w.index + 1) % len(w.clients)
	if w.clients[w.index].Status == "idle" {
		stop, err := bg.Connect(w.clients[w.index].Tg)
		if err != nil {
			return nil, 0, err
		}
		w.clients[w.index].Stop = stop
		w.clients[w.index].Status = "running"
	}
	return w.clients[w.index], w.index, nil
}

var StreamWorkers = &streamWorkers{}
