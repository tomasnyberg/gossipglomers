package main

import (
	"encoding/json"
	"log"
	"os"
	"sync"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	serv := server{n: n, logs: make(map[string][]int), read_to: make(map[string]int)}
	f, _ := os.OpenFile("errlog", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	log.SetOutput(f)
	n.Handle("send", serv.handle_read)
	n.Handle("poll", serv.handle_poll)
	n.Handle("commit_offsets", serv.handle_commit_offsets)
	n.Handle("list_committed_offsets", serv.handle_list_committed_offsets)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n       *maelstrom.Node
	logs    map[string][]int
	read_to map[string]int
	mu      sync.Mutex
}

func (s *server) handle_read(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	val := int(body["msg"].(float64))
	key := body["key"].(string)
	s.logs[key] = append(s.logs[key], val)
	delete(body, "key")
	delete(body, "msg")
	body["type"] = "send_ok"
	body["offset"] = len(s.logs[key]) - 1
	return s.n.Reply(msg, body)
}

func (s *server) handle_poll(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var offsets = body["offsets"].(map[string]interface{})
	var results map[string][][]int = make(map[string][][]int)
	for topic, offset := range offsets {
		offset_inted := int(offset.(float64))
		for idx, msg := range s.logs[topic] {
			// TODO might not want to send everything here in the future. Also, should binary search for the start.
			if idx >= offset_inted {
				results[topic] = append(results[topic], []int{idx, msg})
			}
		}
	}
	body["type"] = "poll_ok"
	body["msgs"] = results
	delete(body, "offsets")
	return s.n.Reply(msg, body)
}

func (s *server) handle_commit_offsets(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var offsets = body["offsets"].(map[string]interface{})
	for topic, offset := range offsets {
		s.read_to[topic] = int(offset.(float64))
	}
	body["type"] = "commit_offsets_ok"
	delete(body, "offsets")
	return s.n.Reply(msg, body)
}

func (s *server) handle_list_committed_offsets(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := body["keys"].([]interface{})
	results := make(map[string]int)
	for _, key := range keys {
		keystring := key.(string)
		if _, ok := s.read_to[keystring]; !ok {
			continue
		}
		results[keystring] = s.read_to[keystring]
	}
	body["offsets"] = results
	body["type"] = "list_committed_offsets_ok"
	delete(body, "keys")
	return s.n.Reply(msg, body)
}
