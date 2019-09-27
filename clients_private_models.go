package promtail

import (
	"time"

	"github.com/alex60217101990/trade_system_websocket_server/logproto"
)

type ClientConfig struct {
	PushURL     string
	ServiceName string
	Labels             string
	BatchWait          time.Duration
	BatchEntriesNumber int
	SendLevel LogLevel
	PrintLevel LogLevel
}

type Label struct {
	Host   string
	Source string
	Job    string
}

type jsonLogEntry struct {
	Ts    time.Time `json:"ts"`
	Line  string    `json:"line"`
	level LogLevel  `json:"-"`
}

type promtailStream struct {
	Labels  string          `json:"labels"`
	Entries []*jsonLogEntry `json:"entries"`
}

type promtailMsg struct {
	Streams []promtailStream `json:"streams"`
}

type protoLogEntry struct {
	entry *logproto.Entry
	level LogLevel
}
