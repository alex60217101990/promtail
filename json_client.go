package promtail

import (
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/eapache/channels"
)

type clientJson struct {
	config     *ClientConfig
	quit       chan struct{}
	entries    *channels.RingChannel
	httpClient *httpClient
	waitGroup  sync.WaitGroup
	logger     *log.Logger
	client     httpClient
	isDebug    bool
}

func NewClientJson(conf ClientConfig, logChanSize int, isDebug bool, logger ...*log.Logger) (Client, error) {
	if len(conf.ServiceName) == 0 {
		return nil, fmt.Errorf("promtail_:_ClientJson_:_NewClientJson: error: empty 'service_name' client config parameter, stack: %s", string(debug.Stack()))
	}
	client := clientJson{
		config:     &conf,
		httpClient: NewHttpClient(),
		quit:       make(chan struct{}),
		entries:    channels.NewRingChannel(channels.BufferCap(logChanSize)),
		isDebug:    isDebug,
	}
	if logger != nil {
		client.logger = logger[0]
	}
	client.SetServiceName(&client.config.ServiceName)
	client.waitGroup.Add(1)
	go client.run()
	return &client, nil
}

func (c *clientJson) Debugf(format string, args ...interface{}) {
	c.log(format, Debug, "Debug: ", args...)
}

func (c *clientJson) Infof(format string, args ...interface{}) {
	c.log(format, Info, "Info: ", args...)
}

func (c *clientJson) Warnf(format string, args ...interface{}) {
	c.log(format, Warn, "Warn: ", args...)
}

func (c *clientJson) Errorf(format string, args ...interface{}) {
	c.log(format, Error, "Error: ", args...)
}

func (c *clientJson) concatStr(format string, prefix string) string {
	line := prefix + fmt.Sprintf("source = %s, time = %s, ", c.config.ServiceName, time.Now().String()) + format
	return line
}

func (c *clientJson) log(format string, level LogLevel, prefix string, args ...interface{}) {
	if (level >= c.config.SendLevel) || (level >= c.config.PrintLevel) {
		c.entries.In() <- &jsonLogEntry{
			Ts:    time.Now(),
			Line:  fmt.Sprintf(c.concatStr(format, prefix), args...),
			level: level,
		}
	}
}

func (c *clientJson) Shutdown() {
	close(c.quit)
	c.waitGroup.Wait()
}

func (c *clientJson) run() {
	var batch []*jsonLogEntry
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
				if entry.(*jsonLogEntry).level >= c.config.PrintLevel && c.isDebug {
					if c.logger != nil {
						c.logger.Print(entry.(*jsonLogEntry).Line)
					} else {
						log.Print(entry.(*jsonLogEntry).Line)
					}
				}
				if entry.(*jsonLogEntry).level >= c.config.SendLevel {
					batch = append(batch, entry.(*jsonLogEntry))
					batchSize++
					if batchSize >= c.config.BatchEntriesNumber {
						c.send(batch)
						batch = []*jsonLogEntry{}
						batchSize = 0
						maxWait.Reset(c.config.BatchWait)
					}
				}
			}
		case <-maxWait.C:
			if batchSize > 0 {
				c.send(batch)
				batch = []*jsonLogEntry{}
				batchSize = 0
			}
			maxWait.Reset(c.config.BatchWait)
		}
	}
}

func (c *clientJson) send(entries []*jsonLogEntry) {
	var streams []promtailStream
	streams = append(streams, promtailStream{
		Labels:  c.config.Labels,
		Entries: entries,
	})
	msg := promtailMsg{Streams: streams}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		if c.isDebug {
			if c.logger != nil {
				c.logger.Printf("promtail_:_ClientJson_:_send_:_[unable to marshal a JSON document] error: %+v, stack: %s\n", err, string(debug.Stack()))
			} else {
				log.Printf("promtail_:_ClientJson_:_send_:_[unable to marshal a JSON document] error: %+v, stack: %s\n", err, string(debug.Stack()))
			}
		}
		return
	}

	resp, body, err := c.httpClient.sendJsonReq("POST", c.config.PushURL, "application/json", jsonMsg)
	if err != nil {
		if c.logger != nil {
			c.logger.Printf("promtail_:_ClientJson_:_send_:_[unable to send an HTTP request] error: %+v, stack: %s\n", err, string(debug.Stack()))
		} else {
			log.Printf("promtail_:_ClientJson_:_send_:_[unable to send an HTTP request] error: %+v, stack: %s\n", err, string(debug.Stack()))
		}
		return
	}

	if resp.StatusCode != 204 {
		if c.logger != nil {
			c.logger.Printf("promtail_:_ClientJson_:_send_: unexpected HTTP status code: %d, message: %s:, stack: %s\n", resp.StatusCode, body, string(debug.Stack()))
		} else {
			log.Printf("promtail_:_ClientJson_:_send_: unexpected HTTP status code: %d, message: %s:, stack: %s\n", resp.StatusCode, body, string(debug.Stack()))
		}
		return
	}
}

func (c *clientJson) SetVersion(version *string) {
	c.config.Labels = strings.Replace(c.config.Labels, "}", ",version=\""+*version+"\"}", len(c.config.Labels)-1)
}

func (c *clientJson) SetServiceName(serviceName *string) {
	c.config.Labels = strings.Replace(c.config.Labels, "}", ",service_name=\""+*serviceName+"\"}", len(c.config.Labels)-1)
}
