package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	serv := server{n: n, values: make(map[int]int), bc: initBroadcast(n, 5)}
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
	mu     sync.Mutex
	bc     broadcaster
}

type broadcastMsg struct {
	peer string
	body []byte
}

type broadcaster struct {
	ch chan broadcastMsg
}

func initBroadcast(n *maelstrom.Node, count int) broadcaster {
	ch := make(chan broadcastMsg)
	for i := 0; i < count; i++ {
		go func() {
			for {
				select {
				case msg := <-ch:
					go func() {
						var body = json.RawMessage(msg.body)
						var success bool = false
						if err := n.RPC(msg.peer, body, func(response_msg maelstrom.Message) error {
							var response_body map[string]interface{}
							if err := json.Unmarshal(response_msg.Body, &response_body); err != nil {
								return err
							}
							if response_body["type"].(string) == "txn_ok" {
								success = true
							} else {
								ch <- msg
							}
							return nil
						}); err != nil {
							ch <- msg
							return
						}
						time.Sleep(250 * time.Millisecond)
						if !success {
							ch <- msg
						}
					}()
				}
			}
		}()
	}
	return broadcaster{ch}
}

func (s *server) handle_txn(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var transactionlist []interface{}
	transactionlist = body["txn"].([]interface{})
	sent_from := s.n.ID()
	if err := body["from"]; err != nil {
		sent_from = body["from"].(string)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var to_send [][]interface{} = make([][]interface{}, 0)
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
		} else if op == "w" {
			s.values[fr] = s.values[int(transaction[2].(float64))] // Note: what if s.values[to] doesn't exist?
			to_send = append(to_send, transaction)
		} else {
			panic(fmt.Errorf("Invalid operation type: %s", op))
		}
	}
	if len(to_send) > 0 {
		s.add_to_broadcast(sent_from, to_send)
	}
	body["type"] = "txn_ok"
	return s.n.Reply(msg, body)
}

func (s *server) add_to_broadcast(sent_from string, to_send [][]interface{}) {
	for _, nbr := range s.n.NodeIDs() {
		if nbr == s.n.ID() || sent_from == nbr { // don't send messages back where they came from
			continue
		}
		var bcmsg broadcastMsg
		bcmsg.peer = nbr
		message_body := map[string]interface{}{
			"type": "txn",
			"txn":  to_send,
			"from": s.n.ID(),
		}
		byte_body, err := json.Marshal(message_body)
		if err != nil {
			log.Fatal(err)
		}
		raw := json.RawMessage(byte_body)
		bcmsg.body = raw
		s.bc.ch <- bcmsg
	}
}
