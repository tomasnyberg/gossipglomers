package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	rand.Seed(time.Now().UnixNano())
	serv := &server{n: n, seen: make([]int, 0, 100)}
	n.Handle("broadcast", serv.receive_broadcast)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n    *maelstrom.Node
	seen []int
}

func (s *server) receive_broadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	num, ok := body["message"].(int)
	if !ok {
		return fmt.Errorf("message is not an int")
	}
	s.seen = append(s.seen, num)
	body["type"] = "broadcast_ok"
	return s.n.Reply(msg, body)
}

func (s *server) read_broadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "read_broadcast_ok"
	body["set"] = s.seen
	return s.n.Reply(msg, body)
}

func (s *server) topology(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "topology_ok"
	return s.n.Reply(msg, body)
}
