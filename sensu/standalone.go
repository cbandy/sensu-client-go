package sensu

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/upfluence/sensu-client-go/sensu/check"
)

type Standalone struct {
	Client    *Client
	closeChan chan bool

	Check    check.Check
	Interval time.Duration
	Name     string
}

func NewStandalone(name string, attributes map[string]interface{}) *Standalone {
	s := Standalone{Name: name, closeChan: make(chan bool)}

	if command, ok := attributes["command"]; ok {
		s.Check = &check.ExternalCheck{command.(string)}
	}
	if interval, ok := attributes["interval"]; ok {
		s.Interval = time.Duration(interval.(float64)) * time.Second
	}

	return &s
}

// SetClient implements the Processor interface.
func (s *Standalone) SetClient(c *Client) error {
	s.Client = c
	return nil
}

// Start implements the Processor interface.
func (s *Standalone) Start() error {
	if s.Check == nil {
		return fmt.Errorf("No command defined for Standalone %q", s.Name)
	}
	if s.Interval <= 0 {
		return fmt.Errorf("No interval defined for Standalone %q", s.Name)
	}

	ticker := time.NewTicker(s.Interval)

	for {
		select {
		case t := <-ticker.C:
			if err := s.run(t); err != nil {
				log.Printf("Unable to send results for Standalone %q: %v", s.Name, err)
			}
		case <-s.closeChan:
			ticker.Stop()
			return nil
		}
	}
}

// Close implements the Processor interface.
func (s *Standalone) Close() {
	s.closeChan <- true
}

func (s *Standalone) run(issued time.Time) (err error) {
	output := s.Check.Execute()
	result := map[string]interface{}{
		"client": s.Client.Config.Name(),
		"check": map[string]interface{}{
			"name":     s.Name,
			"issued":   issued.Unix(),
			"output":   output.Output,
			"duration": output.Duration,
			"status":   output.Status,
			"executed": output.Executed,
		},
	}

	if payload, err := json.Marshal(result); err == nil {
		err = s.Client.Transport.Publish("direct", "results", "", payload)
	}

	return
}
