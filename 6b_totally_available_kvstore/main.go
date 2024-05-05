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
	serv := server{n: n, values: make(map[int]int)}
	f, _ := os.OpenFile("errlog", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	log.SetOutput(f)
	n.Handle("txn", serv.handle_txn)
	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n      *maelstrom.Node
	values map[int]int
	mu      sync.Mutex
}

func (s *server) handle_txn(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var transactionlist []interface{}
	transactionlist = body["txn"].([]interface{})
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, transaction_uncast := range transactionlist {
		var transaction []interface{}
		transaction = transaction_uncast.([]interface{})
		var op string
		var fr int
		op, fr = transaction[0].(string), int(transaction[1].(float64))
		// TODO make this a function
		if op == "r" {
			value, ok := s.values[fr]
			if !ok {
				transaction[2] = s.values[fr]
			} else {
				transaction[2] = value
			}
		} else {
			s.values[fr] = s.values[int(transaction[2].(float64))] // Note: what if s.values[to] doesn't exist?
		}
	}

	body["type"] = "txn_ok"
	return s.n.Reply(msg, body)
}
