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
	serv := &server{n: n, seen_set: make(map[int]struct{})}
	serv.broadcast_worker = initBroadcast(n, 10, serv)
	n.Handle("broadcast", serv.receive_broadcast)
	n.Handle("read", serv.read_broadcast)
	n.Handle("topology", serv.receive_topology)
	n.Handle("multi_broadcast", serv.receive_multi_broadcast)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type broadcastMsg struct {
	peer string
	body []byte
}

type broadcaster struct {
	ch chan string
	server *server
}

type neighbor struct {
	neighbor_mutex *sync.Mutex
	to_send   map[int]struct{} // values we need to send to our neighbor
}

type server struct {
	n                *maelstrom.Node
	seen_set         map[int]struct{}
	seen_mutex       sync.Mutex
	neighbors        map[string]neighbor
	broadcast_worker broadcaster
}

func initBroadcast(n *maelstrom.Node, count int, s *server) broadcaster {
	ch := make(chan string)
	for i := 0; i < count; i++ {
		go func() {
			for {
				select {
					case nbr := <-ch:
						go func() {
							var success bool = false
							s.neighbors[nbr].neighbor_mutex.Lock()
							defer s.neighbors[nbr].neighbor_mutex.Unlock()
							// Save s.neighbors[nbr].to_send to a []int instead of a map
							var messages []int = make([]int, 0)
							for value := range s.neighbors[nbr].to_send {
								messages = append(messages, value)
							}
							message_body := map[string]interface{}{
								"message": messages,
								"type":    "multi_broadcast",
							}
							message_body_byte, err := json.Marshal(message_body)
							if err != nil {
								log.Fatal(err)
							}
							message_body_raw := json.RawMessage(message_body_byte)
							if err := n.RPC(nbr, message_body_raw, func(response_msg maelstrom.Message) error {
								var response_body map[string]interface{}
								if err := json.Unmarshal(response_msg.Body, &response_body); err != nil {
									return err
								}
								if response_body["type"].(string) == "broadcast_ok" {
									success = true
									// Remove the messages we sent from the to_send list
									for value := range s.neighbors[nbr].to_send {
										delete(s.neighbors[nbr].to_send, value)
									}
								}
								return nil
							}); err != nil {
								ch <- nbr
							}
							time.Sleep(250 * time.Millisecond)
							if !success {
								ch <- nbr
							}
						}()
					}
			}
		}()
	}
	return broadcaster{ch, s}
}

func (s *server) receive_multi_broadcast(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var messages []int = make([]int, 0)
	for _, message := range body["message"].([]interface{}) {
		messages = append(messages, int(message.(float64)))
	}
	s.handle_new_messages(messages)
	delete(body, "message")
	body["type"] = "broadcast_ok"
	return s.n.Reply(msg, body)
}

func (s *server) handle_new_messages(messages[] int){
	s.seen_mutex.Lock()
	var to_send []int = make([]int, 0)
	for _ , message := range messages {
		if _, ok := s.seen_set[message]; !ok {
			s.seen_set[message] = struct{}{}
			to_send = append(to_send, message)
		}
	}
	s.seen_mutex.Unlock()
	for peer := range s.neighbors {
		s.neighbors[peer].neighbor_mutex.Lock()
		for _, message := range to_send {
			s.neighbors[peer].to_send[message] = struct{}{}
		}
		s.broadcast_worker.ch <- peer
		s.neighbors[peer].neighbor_mutex.Unlock()
	}
}

func (s *server) receive_broadcast(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var received_int int = int(body["message"].(float64))
	// create an int array of just the received int
	var messages []int = []int{received_int}
	s.handle_new_messages(messages)
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
				s.neighbors[nbr.(string)] = neighbor{to_send: make(map[int]struct{}), neighbor_mutex: &sync.Mutex{}}
			}
		}
	}
	delete(body, "topology")
	body["type"] = "topology_ok"
	return s.n.Reply(msg, body)
}
