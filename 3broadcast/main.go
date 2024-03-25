package main

import (
	"encoding/json"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	serv := &server{n: n, seen: make([]int, 0, 100)}
	n.Handle("broadcast", serv.receive_broadcast)
	n.Handle("read", serv.read_broadcast)
	n.Handle("topology", serv.receive_topology)

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	n    *maelstrom.Node
	seen []int
	topology map[string]map[string]struct{}
}

// TODO remove and use set
func exists(slice []int, element int) bool {
    for _, v := range slice {
        if v == element {
            return true
        }
    }
    return false
}

func (s *server) receive_broadcast(msg maelstrom.Message) error {
	var body map[string]interface{}
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	var received_int int = int(body["message"].(float64))
	if !exists(s.seen, received_int) {
		s.seen = append(s.seen, received_int)
		msg_body := map[string]interface{}{
			"message": received_int,
			"type": "broadcast",
			"node_generated": true,
		}
		msg_body_byte, err := json.Marshal(msg_body)
		if err != nil {
			return err
		}
		// Print the msg_body_byte to stderr
		// log.Printf("msg_body_byte: %s\n", msg_body_byte)
		for peer := range s.topology[s.n.ID()] {
			s.n.Send(peer, json.RawMessage(msg_body_byte))
		}
	}
	// Check if "node_generated" exists in the message body
	if _, ok := body["node_generated"]; ok {
		return nil
	}
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
	body["messages"] = s.seen
	return s.n.Reply(msg, body)
}

func (s *server) receive_topology(msg maelstrom.Message) error {
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}
	s.topology = make(map[string]map[string]struct{})
	for k, v := range body["topology"].(map[string]interface{}) {
		s.topology[k] = make(map[string]struct{})
		for _, peer := range v.([]interface{}) {
			s.topology[k][peer.(string)] = struct{}{}
		}
	}
	delete(body, "topology")
	body["type"] = "topology_ok"
	return s.n.Reply(msg, body)
}
