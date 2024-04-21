package main

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	// "fmt"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type server struct {
	n   *maelstrom.Node
	kv  *maelstrom.KV
	ctx *context.Context
	mu  sync.Mutex
	cache map[string]int
}

func main() {
	n := maelstrom.NewNode()
	ctx := context.Background()
	kv := maelstrom.NewSeqKV(n)
	serv := &server{n: n, kv: kv, ctx: &ctx, mu:sync.Mutex{}, cache:make(map[string]int)}
	f, _ := os.OpenFile("errlog", os.O_RDWR | os.O_CREATE | os.O_TRUNC, 0666)
	log.SetOutput(f)
	n.Handle("read", serv.accept_read)
	n.Handle("add", serv.accept_add)
	n.Handle("local", serv.accept_local)
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
	var value int = 0
	for _, id := range(s.n.NodeIDs()) {
		var peer_val int = s.cache[id]
		if id == s.n.ID() {
			read_res, read_res_err := s.kv.ReadInt(*s.ctx, id)
			if read_res_err == nil {
				peer_val = read_res
			}
		} else {
			ctx, cancel := context.WithTimeout(*s.ctx, time.Second)
			body := map[string]any {
				"type": "local",
			}
			msg, msg_err := s.n.SyncRPC(ctx, id, body)
			cancel()
			var res_body map[string]any
			if parse_err := json.Unmarshal(msg.Body, &res_body); parse_err != nil {
				log.Printf("Incorrect response from local %s \n", msg_err)
				// return parse_err
			} else {
				peer_val = int(res_body["value"].(float64))
			}
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
	value := s.cache[s.n.ID()]
	s.kv.Write(*s.ctx, s.n.ID(), value + delta)
	s.cache[s.n.ID()] = value + delta
	delete(body, "delta")
	return s.n.Reply(msg, body)
}

func (s *server) accept_local(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "local_ok"
	body["value"] = s.cache[s.n.ID()]
	return s.n.Reply(msg, body)
}
