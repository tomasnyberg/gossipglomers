## General
- Only messages on STDOUT, debug on STDERR
- Separate messages by \n
- n.Run() for a node fires off a goroutine
- n.Send() sends a fire-and-forget message (no response expected)
- n.RPC() sends a message and expectes a response, which you supply a handler for.