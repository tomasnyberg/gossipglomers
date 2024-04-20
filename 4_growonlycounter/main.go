package main

import (
	"context"
	"encoding/json"
	// "fmt"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type server struct {
	n   *maelstrom.Node
	kv  *maelstrom.KV
	ctx *context.Context
}

func main() {
	n := maelstrom.NewNode()
	ctx := context.Background()
	kv := maelstrom.NewSeqKV(n)
	serv := &server{n: n, kv: kv, ctx: &ctx}
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
	value, err := s.kv.ReadInt(*s.ctx, "value")
	if err != nil {
		value = 0
		// return err;
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
	value, err := s.kv.ReadInt(*s.ctx, "value")
	if err != nil {
		value = 0
		// return err;
	}
	s.kv.CompareAndSwap(*s.ctx, "value", value, value+delta, true)
	delete(body, "delta")
	return s.n.Reply(msg, body)
}
