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

type broadcaster struct {
	ch     chan string
	server *server
}

type neighbor struct {
	neighbor_mutex *sync.Mutex
	to_send        map[int]struct{} // values we need to send to our neighbor
}

type server struct {
	n                *maelstrom.Node
	seen_set         map[int]struct{}
	seen_mutex       sync.Mutex
	neighbors        map[string]neighbor
	broadcast_worker broadcaster
}

func multi_broadcast_to_nbr(s *server, n *maelstrom.Node, nbr string, ch chan string) {
	var success bool = false
	s.neighbors[nbr].neighbor_mutex.Lock()
	if len(s.neighbors[nbr].to_send) == 0 {
		s.neighbors[nbr].neighbor_mutex.Unlock()
		return
	}
	var messages []int = make([]int, 0)
	for value := range s.neighbors[nbr].to_send {
		messages = append(messages, value)
		delete(s.neighbors[nbr].to_send, value)
	}
	s.neighbors[nbr].neighbor_mutex.Unlock()
	message_body := map[string]interface{}{
		"message": messages,
		"type":    "multi_broadcast",
	}
	message_body_byte, err := json.Marshal(message_body)
	if err != nil {
		log.Fatal(err)
	}
	message_body_raw := json.RawMessage(message_body_byte)
	err = n.RPC(nbr, message_body_raw, func(response_msg maelstrom.Message) error {
		var response_body map[string]interface{}
		if err := json.Unmarshal(response_msg.Body, &response_body); err != nil {
			return err
		}
		if response_body["type"].(string) == "broadcast_ok" {
			success = true
		}
		return nil
	})
	time.Sleep(250 * time.Millisecond)
	if !success || err != nil {
		s.neighbors[nbr].neighbor_mutex.Lock()
		for _, message := range messages {
			s.neighbors[nbr].to_send[message] = struct{}{}
		}
		s.neighbors[nbr].neighbor_mutex.Unlock()
		ch <- nbr
	}
}

func initBroadcast(n *maelstrom.Node, count int, s *server) broadcaster {
	ch := make(chan string)
	for i := 0; i < count; i++ {
		go func() {
			for {
				select {
				case nbr := <-ch:
					go multi_broadcast_to_nbr(s, n, nbr, ch)
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
	s.handle_new_messages(messages, msg.Src)
	delete(body, "message")
	body["type"] = "broadcast_ok"
	return s.n.Reply(msg, body)
}

func (s *server) handle_new_messages(messages []int, src string) {
	s.seen_mutex.Lock()
	var to_send []int = make([]int, 0)
	for _, message := range messages {
		if _, ok := s.seen_set[message]; !ok {
			s.seen_set[message] = struct{}{}
			to_send = append(to_send, message)
		}
	}
	s.seen_mutex.Unlock()
	for peer := range s.neighbors {
		if peer == src {
			continue
		}
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
	s.handle_new_messages(messages, msg.Src)
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
	// Every node does this which is a bit excessive, but for an actual system we would supply this when initializing the network.
	// In fact, that is possible by passing --topology tree4 to the maelstrom command, but I felt like that was cheating.
	topology := generate_tree5_topology(s.n)
	for _, nbr := range topology[s.n.ID()] {
		s.neighbors[nbr] = neighbor{to_send: make(map[int]struct{}), neighbor_mutex: &sync.Mutex{}}
	}
	delete(body, "topology")
	body["type"] = "topology_ok"
	return s.n.Reply(msg, body)
}

func generate_tree5_topology(n *maelstrom.Node) (topology map[string][]string) {
	ids := n.NodeIDs()
	topology = make(map[string][]string)
	for _, id := range ids {
		topology[id] = []string{}
	}
	st := []string{"n0"}
	in_tree := make(map[string]struct{})
	for len(st) > 0 {
		// This is inefficient, but I don't care :) we're just gonna 25 nodes anyway
		// a realistic cluster probably wouldn't exceed that anyway [CITATION NEEDED]
		node := st[0]
		st = st[1:]
		for _, id := range ids {
			if _, ok := in_tree[id]; !ok {
				topology[node] = append(topology[node], id)
				topology[id] = append(topology[id], node)
				in_tree[id] = struct{}{}
				st = append(st, id)
			}
			if len(topology[node]) == 5 {
				break
			}
		}
	}
	return
}
