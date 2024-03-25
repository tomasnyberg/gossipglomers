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