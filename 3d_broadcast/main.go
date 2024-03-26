package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	serv := &server{n: n, seen_set: make(map[int]struct{}), broadcast_worker: initBroadcast(n, 15)}
	n.Handle("broadcast", serv.receive_broadcast)
	n.Handle("read", serv.read_broadcast)
	n.Handle("topology", serv.receive_topology)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type broadcastMsg struct {
	peer string
	body []byte
}

type broadcaster struct {
	ch chan broadcastMsg
}

type neighbor struct {
	neighbor_mutex sync.Mutex
	to_send   map[int]struct{} // values we need to send to our neighbor
}

type server struct {
	n                *maelstrom.Node
	seen_set         map[int]struct{}
	seen_mutex       sync.Mutex
	neighbors        map[string]neighbor
	broadcast_worker broadcaster
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
							if response_body["type"].(string) == "broadcast_ok" {
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

func (s *server) receive_broadcast(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var received_int int = int(body["message"].(float64))
	s.seen_mutex.Lock()
	// If we have not seen the message before, keep track of it and broadcast it to all peers.
	if _, ok := s.seen_set[received_int]; !ok {
		s.seen_set[received_int] = struct{}{}
		msg_body := map[string]interface{}{
			"message":        received_int,
			"type":           "broadcast",
			"node_generated": true,
		}
		msg_body_byte, err := json.Marshal(msg_body)
		if err != nil {
			return err
		}
		for peer := range s.neighbors {
			var msg broadcastMsg = broadcastMsg{peer, msg_body_byte}
			s.broadcast_worker.ch <- msg
		}
	}
	s.seen_mutex.Unlock()
	delete(body, "message")
	body["type"] = "broadcast_ok"
	return s.n.Reply(msg, body)
}

func (s *server) read_broadcast(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	body["type"] = "read_ok"
	s.seen_mutex.Lock()
	// Convert the set to a list and send that back.
	var result []int = make([]int, 0, len(s.seen_set))
	for id := range s.seen_set {
		result = append(result, id)
	}
	s.seen_mutex.Unlock()
	body["messages"] = result
	return s.n.Reply(msg, body)
}

func (s *server) receive_topology(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.neighbors = make(map[string]neighbor)
	for k, v := range body["topology"].(map[string]interface{}) {
		// If the key is this node, add all the neighbors to the topology map, along with an empty list as their value
		if k == s.n.ID() {
			for _, nbr := range v.([]interface{}) {
				s.neighbors[nbr.(string)] = neighbor{to_send: make(map[int]struct{})}
			}
		}
	}
	delete(body, "topology")
	body["type"] = "topology_ok"
	return s.n.Reply(msg, body)
}
