package main

import (
	"encoding/json"
	// "fmt"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

type server struct {
	n     *maelstrom.Node
	value int
}

func main() {
	n := maelstrom.NewNode()
	serv := &server{n: n, value: 0}
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
	body["value"] = s.value
	return s.n.Reply(msg, body)
}

func (s *server) accept_add(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "add_ok"
	var delta int = int(body["delta"].(float64))
	s.value += delta
	delete(body, "delta")
	return s.n.Reply(msg, body)
}
