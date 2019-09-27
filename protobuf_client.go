package promtail

import (
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/alex60217101990/trade_system_websocket_server/logproto"
	"github.com/eapache/channels"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/snappy"
)

type clientProto struct {
	config    *ClientConfig
	quit      chan struct{}
	entries   *channels.RingChannel
	waitGroup sync.WaitGroup
	client    *httpClient
	isDebug   bool
	logger    *log.Logger
}

func NewClientProto(conf ClientConfig, logChanSize int, isDebug bool, logger ...*log.Logger) (Client, error) {
	if len(conf.ServiceName) == 0 {
		return nil, fmt.Errorf("promtail_:_ClientJson_:NewClientProto: error: empty 'service_name' client config parameter, stack: %s", string(debug.Stack()))
	}
	client := clientProto{
		config:  &conf,
		quit:    make(chan struct{}),
		entries: channels.NewRingChannel(channels.BufferCap(logChanSize)),
		client:  NewHttpClient(),
		isDebug: isDebug,
	}
	if logger != nil {
		client.logger = logger[0]
	}
	client.SetServiceName(&client.config.ServiceName)
	client.waitGroup.Add(1)
	go client.run()
	return &client, nil
}

func (c *clientProto) Debugf(format string, args ...interface{}) {
	c.log(format, Debug, "Debug: ", args...)
}

func (c *clientProto) Infof(format string, args ...interface{}) {
	c.log(format, Info, "Info: ", args...)
}

func (c *clientProto) Warnf(format string, args ...interface{}) {
	c.log(format, Warn, "Warn: ", args...)
}

func (c *clientProto) Errorf(format string, args ...interface{}) {
	c.log(format, Error, "Error: ", args...)
}

func (c *clientProto) concatStr(format string, prefix string) string {
	return prefix + fmt.Sprintf("source = %s, time = %s, ", c.config.ServiceName, time.Now().String()) + format
}

func (c *clientProto) log(format string, level LogLevel, prefix string, args ...interface{}) {
	if (level >= c.config.SendLevel) || (level >= c.config.PrintLevel) {
		now := time.Now().UnixNano()
		c.entries.In() <- &protoLogEntry{
			entry: &logproto.Entry{
				Timestamp: &timestamp.Timestamp{
					Seconds: now / int64(time.Second),
					Nanos:   int32(now % int64(time.Second)),
				},
				Line: fmt.Sprintf(c.concatStr(format, prefix), args...),
			},
			level: level,
		}
	}
}

func (c *clientProto) Shutdown() {
	close(c.quit)
	c.waitGroup.Wait()
}

func (c *clientProto) run() {
	var batch []*logproto.Entry
	batchSize := 0
	maxWait := time.NewTimer(c.config.BatchWait)
	defer func() {
		if batchSize > 0 {
			c.send(batch)
		}
		c.waitGroup.Done()
	}()

	for {
		select {
		case <-c.quit:
			return
		case entry, ok := <-c.entries.Out():
			if ok {
				if entry.(*protoLogEntry).level >= c.config.PrintLevel && c.isDebug {
					if c.logger != nil {
						c.logger.Print(entry.(*protoLogEntry).entry.Line)
					} else {
						log.Print(entry.(*protoLogEntry).entry.Line)
					}
				}
				if entry.(*protoLogEntry).level >= c.config.SendLevel {
					batch = append(batch, entry.(*protoLogEntry).entry)
					batchSize++
					if batchSize >= c.config.BatchEntriesNumber {
						c.send(batch)
						batch = []*logproto.Entry{}
						batchSize = 0
						maxWait.Reset(c.config.BatchWait)
					}
				}
			}
		case <-maxWait.C:
			if batchSize > 0 {
				c.send(batch)
				batch = []*logproto.Entry{}
				batchSize = 0
			}
			maxWait.Reset(c.config.BatchWait)
		}
	}
}

func (c *clientProto) send(entries []*logproto.Entry) {
	var streams []*logproto.Stream
	streams = append(streams, &logproto.Stream{
		Labels:  c.config.Labels,
		Entries: entries,
	})
	req := logproto.PushRequest{
		Streams: streams,
	}
	buf, err := proto.Marshal(&req)
	if err != nil {
		if c.isDebug {
			if c.logger != nil {
				c.logger.Printf("promtail_:_ClientProto_:_send_:_[unable to marshal] error: %+v, stack: %s\n", err, string(debug.Stack()))
			} else {
				log.Printf("promtail_:_ClientProto_:_send_:_[unable to marshal] error: %+v, stack: %s\n", err, string(debug.Stack()))
			}
		}
		return
	}
	buf = snappy.Encode(nil, buf)
	resp, body, err := c.client.sendJsonReq("POST", c.config.PushURL, "application/x-protobuf", buf)
	if err != nil {
		if c.logger != nil {
			c.logger.Printf("promtail_:_ClientProto_:_send_:_[unable to send an HTTP request] error: %+v, stack: %s\n", err, string(debug.Stack()))
		} else {
			log.Printf("promtail_:_ClientProto_:_send_:_[unable to send an HTTP request] error: %+v, stack: %s\n", err, string(debug.Stack()))
		}
		return
	}
	if resp.StatusCode != 204 {
		if c.logger != nil {
			c.logger.Printf("promtail_:_ClientProto_:_send_: unexpected HTTP status code: %d, message: %s:, stack: %s\n", resp.StatusCode, body, string(debug.Stack()))
		} else {
			log.Printf("promtail_:_ClientProto_:_send_: unexpected HTTP status code: %d, message: %s:, stack: %s\n", resp.StatusCode, body, string(debug.Stack()))
		}
		return
	}
}

func (c *clientProto) SetVersion(version *string) {
	c.config.Labels = strings.Replace(c.config.Labels, "}", ",version=\""+*version+"\"}", len(c.config.Labels)-1)
}

func (c *clientProto) SetServiceName(serviceName *string) {
	c.config.Labels = strings.Replace(c.config.Labels, "}", ",service_name=\""+*serviceName+"\"}", len(c.config.Labels)-1)
}
