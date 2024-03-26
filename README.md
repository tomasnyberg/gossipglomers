# Gossip Glomers
My (attempts at) solutions for the Gossip Glomers Distributed Systems challenge by fly.io. Some short explanations of the challenges as well as my approaches to solving them below.

## 1: Echo
For this challenge, the task is simply to have your nodes respond to an incoming message with the same information that they received.
#### Solution
I did not do much of interest here beyond copying the sample code; the task mainly exists to get you started with Maelstrom and the libraries.

## 2: Unique ID Generation 
For this challenge, you are supposed to respond to incoming requests with a globally unique ID.
This does not necessarily require any communication between the nodes, and simply generating an ID that is (very) unlikely to collide with an existing one is sufficient.
#### Solution
 I simply took the current UNIX timestamp (in nanoseconds), and generated a random integer in the range `[0, 2^63 - 1[`.
  Concatenating these two gave me something that felt reasonably unlikely to ever result in a collision.
 I did use the standard rand library however, so perhaps there is some exploit that remains available with this approach. 

## 3: Broadcast
This challenge is split up into 5 parts (a-e) but the underlying idea is the same: one node receives an integer from Maelstrom, and every node needs to keep track of every integer that has been sent into the network.
### 3a: Single-Node Broadcast
To start things off, the only requirement is to make sure that one node can keep track of everything it has seen. 
#### Solution
For this first part, I simply saved all the nodes to a list and wrangled the JSON into the correct format.

### 3b: Multi-Node Broadcast
As the name suggests, we now have multiple nodes that need to work together, forcing us to actually do some communication between them.
#### Solution
Since the network is always available, and we can assume every message we send between nodes always arrives, we simply need to inform all of our neighbors when we see a new integer.
We can do this by simply checking if we have seen an integer when it arrives, and if we haven't, we send it to all our peers (using the "fire-and-forget" Send() method).
 I used the topology supplied by Maelstrom at the start of the simulation to decide who talks to who.
 
 Given the frequent occurrence of the operation "have I seen this integer?" I opted for a set in each node as opposed to the simple list I was previously using. 
However, because n.Handle fires off a goroutine for each incoming request, this led to some issues with concurrent access to said set. 
A simple mutex was thus required, after which my nodes could happily share and receive integers without worrying about concurrent access. 

### 3c: Fault-Tolerant Broadcast
Now, network paritions are introduced, meaning nodes occasionally cannot send messages to each other.

#### Solution
Our previous approach will not work under this condition since we cannot assume that a node actually receives a sent message.
 Some form of retrying and resending messages is now required.
  For that purpose, I introduced a `broadcaster`, whose job is simply to manage a few (I went with 10) goroutines that look at which messages need to be processed, and send them off using the `RPC` call.
   The tasks for a single goroutine looks something like this:
```
0: Wait for a message that needs to be sent
1: Take a message that needs to be sent off the channel in the broadcaster
2: Spin up another goroutine that sends the message and waits for it to arrive.
3: If the message was not acknowledged by the receiving node within 250ms, place it back into the channel so we can retry.
4: GOTO 0
```
While this approach solves the problem, allowing nodes to propagate even with a faulty network, it remains very inefficient. Inspecting the `results.edn` file, we see the following:
```edn 
 :net {:all {:send-count 23971,
             :recv-count 2502,
             :msg-count 23971,
             :msgs-per-op 119.258705},
       :clients {:send-count 422, :recv-count 422, :msg-count 422},
       :servers {:send-count 23549,
                 :recv-count 2080,
                 :msg-count 23549,
                 :msgs-per-op 117.1592},
```
which is to say, we send more than `100` messages around the network for every new message that enters it (not great). Performance was of fairly small concern here, but that will change in the upcoming challenge(s). There is also this issue:
```edn
:stable-count 91,
:stale-count 79,
```
what this means is that, while all our value were stable, stale behaviour was observed in 79 (almost all) of them. This suggests that, while the values are being propagated correctly, they do so very slowly.