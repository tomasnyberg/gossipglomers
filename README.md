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
which is to say, we send more than `100` messages around the network for every new message that enters it (not great). Performance was of fairly small concern here, but that will change in the upcoming challenge(s).

### 3d: Efficient broadcast, part 1
In the words of the challenge authors: "<em>Not only do [Distributed Systems] need to be correct, but they also need to be fast</em>." Where as before we were only concerned with correctness, we are now tasked with making the network efficient as well. In addition to ensuring our solution is correct (including with network outages!) we now need to meet certain performance criteria.

The paremeters for this challenge are bumped up quite significantly:
```
Nodes in the cluster: 25
Latency: 100ms
Rate (new messages/s): 100
```
And the performance criteria are as follows:
```
Messages per operation: Less than 30
Median latency: Less than 400
Max latency: Less than 600
```
Running my (unmodified) 3c solution with these parameters resulted in the following metrics:
```
Messages per operation: ~80
Median latency: ~450
Max latency: ~800
```
which is to say, some drastic improvements were needed.

#### Solution
The first inefficiency that came to my mind was the fact that we always send one message at a time, regardless of if the neighbor we're sending it to has multiple unseen messages. My idea to resolve this was fairly simple (and much more complicated to implement): For all our neighbors, keep track of the messages they have not seen yet. Then, when sending messages to that neighbor, send everything in that set. See the method `multi_broadcast_to_nbr()` method for reference.

While this reduced the messages per operation by a good amount (around 20-30%), it was not at all as efficient as I imagined it to be. 
Upon closer inspection, a significant portion of messages being sent contained only one message regardless, meaning there was oftentimes no improvement.  
Nonetheless, it was a big improvement and moved me closer to the goal.
I would also imagine that it's more useful in the case of network failures, where a node has to wait a while before talking to its neighbors (letting multiple messages pile up).

A <strong>much</strong> simpler improvement that I thought of shortly afterwards was even more effective, more than halving the messages per operation metric.
It was as simple as this: If a node receives a message from a neighbor, it should not send that message back to the same neighbor.

The last thing that I improved was simply changing the topology of the network. The default for maelstrom is a 2d grid, which is of course quite inefficient when passing messages from one corner to the other. 
I noticed that `--topology tree4`, which changes the default topology that maelstrom provides to a tree with branching factor `4` offered significant improvements to all targeted metrics. In fact, enough to make me pass all the requirements.

Obviously, I was not satisfied with just passing a paremeter, so I implemented the construction of the tree myself. Using a tree (with branching factor 5, since I found that was even better) I finally obtained these results:
```
Messages per operation: ~19
Median latency: ~375
Max latency: ~500
```
which meant that I had completed the challenge 🚀

### 3e: Efficient broadcast, part 2
<em>"Why settle for a fast distributed system when you could always make it faster?"</em>

The next challenge does not impose any additional constraints, but simply adjusts the performance criteria that need to be met.
The metrics that need to be met for this challenge (with the same network parameters as before) are:
```
Messages per operation: Less than 20
Median latency: Less than 1000ms
Max latency: Less than 2000ms
```
#### Solution
Funnily enough, my solution to the previous task already meets all of the criteria, with the messages per operation metric just barely squeezing by. At this point, I was getting fairly tired of gossiping integers around, and so I did not bother optimizing my network further. I'm guessing some of the optimizations I did in 3d were intended for 3e (such as the multi-broadcasting).

## 4: Grow-only counter
In this challenge, we have three nodes in the cluster, and they periodically receive messages of this form:
```yaml
type:add
delta:123
```
The challenge is then to keep track of the total sum of all deltas that have entered the network, and ensure that it is <strong>eventually</strong> consistent. Which is to say, every node periodically has to answer messages of this form:
```
type:read
```
with 
```
value:x // Where x is the current global total of all the deltas
```
We are also given access to a <strong>sequentially</strong> consistent KV-store that maelstrom manages for us.
#### Solution.
The challenge is to create a [Conflict-free replicated datatype](https://en.wikipedia.org/wiki/Conflict-free_replicated_data_type). A naive approach would be to, on every add operation, read the value in the KV-store and write back the updated value. Because of the check-then-act nature of this, it obviously won't succeed. A slightly better approach is to read and perform compare-and-swap until the swap succeeds, which ensures that updates aren't overwriting each other. While this is slightly better and ensures updates aren't lost, it doesn't guarantee eventual consistency due to the amount of contention and inefficiency. For reference, I would get correct results about 50% of the time with this approach.

Slightly better is to have a value in the KV for every node, and then the answer for a read message would simply be the sum of the KV-reads for every node. At this point, I also introduced a local cache for every node which keeps track of the values of other nodes (in the case a KV-read isn't successful due to network partitions). This actually succeeded most of the time for me, with about an 80% success rate for the given parameters. The remaining issue was still the slow convergence.

The final improvement I made was to, instead of asking the KV for every node's value, ask the other nodes themselves for it. This improved my convergence to the point where my tests would pass 100% of the time (or at least it seems that way, based on 50 runs without any errors). 

## 5: Kafka-style log
This challenge, like challenge 3, consists of a series of sub-challenges that get increasingly harder. The idea is to create a replicated log service similar to Kafka (hence Kafka-style).

### 5a: Single-node
To get things started, we are asked simply be able to support all the commands that our nodes will be called with in future challenges, and it only needs to work for a single node.
The commands are:
```yaml
send: Append a message (int) to a topic and respond with the offset for the message.
poll: Return messages for a set of topics starting from some offset
commit_offsets: informs our node that messages have been successfully processed up to and including some offset
list_committed_offsets: Return a map of committed offsets for a given set of topics
```
#### Solution
Implementing this was basically just an exercise in following the instructions, as there is no real complexity with a single node. I did run into a mysterious error that had my goroutines spitting out cryptic messages to STDERR however. That error turned out to be a simple concurrent access problem, and was easily fixed with a mutex. 