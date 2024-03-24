package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"
	"fmt"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)


func main() {
	n := maelstrom.NewNode()
	rand.Seed(time.Now().UnixNano())
	serv := &server{n: n}
	n.Handle("generate", serv.generate)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n *maelstrom.Node
}

func (s *server) generate(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var num int64 = rand.Int63n((1 << 63) - 1)
	var id string = fmt.Sprintf("%d%s%d", time.Now().UnixNano(), s.n.ID()[1:], num)
	body["type"] = "generate_ok"
	body["id"] = id
	return s.n.Reply(msg, body)
}
