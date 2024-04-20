package main

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	// "fmt"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type server struct {
	n   *maelstrom.Node
	kv  *maelstrom.KV
	ctx *context.Context
	mu  sync.Mutex
}

func main() {
	n := maelstrom.NewNode()
	ctx := context.Background()
	kv := maelstrom.NewSeqKV(n)
	serv := &server{n: n, kv: kv, ctx: &ctx, mu:sync.Mutex{}}
	f, _ := os.OpenFile("errlog", os.O_RDWR | os.O_CREATE | os.O_TRUNC, 0666)
	log.SetOutput(f)
	n.Handle("read", serv.accept_read)
	n.Handle("add", serv.accept_add)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

func (s *server) accept_read(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "read_ok"
	s.mu.Lock()
	defer s.mu.Unlock()
	var value int = 0
	for _, id := range(s.n.NodeIDs()) {
		peer_val, err := s.kv.ReadInt(*s.ctx, id)
		if err != nil {
			peer_val = 0
		}
		value += peer_val
	}
	body["value"] = value
	return s.n.Reply(msg, body)
}

func (s *server) accept_add(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "add_ok"
	var delta int = int(body["delta"].(float64))
	s.mu.Lock()
	defer s.mu.Unlock()
	value, err := s.kv.ReadInt(*s.ctx, s.n.ID())
	if err != nil {
		value = 0
	}
	s.kv.Write(*s.ctx, s.n.ID(), value + delta)
	delete(body, "delta")
	return s.n.Reply(msg, body)
}
