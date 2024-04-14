import networkx as nx
import matplotlib.pyplot as plt
from collections import deque

def display_graph(nodes, edges):
    G = nx.Graph()
    G.add_nodes_from(nodes)
    G.add_edges_from(edges)
    nx.draw(G, with_labels=True, node_color='skyblue', node_size=1500, font_size=12, font_weight='bold')
    plt.show()

N = 25

def n_tree(N):
    adj_list = {i:[] for i in range(1, N+1)}
    q = deque([1])
    in_tree = set([1])
    while q:
        n = q.pop()
        for i in range(1, N+1):
            if i not in in_tree:
                adj_list[n].append(i)
                adj_list[i].append(n)
                in_tree.add(i)
                q.appendleft(i)
            if len(adj_list[n]) == 4:
                break
    return adj_list

def generate_nodes_and_edges(adj_list):
    nodes = [x for x in adj_list]
    edges = []
    for node in adj_list:
        for neighbor in adj_list[node]:
            edges.append((node, neighbor))
    return nodes, edges

adj_list = n_tree(N)
nodes, edges = generate_nodes_and_edges(adj_list)
display_graph(nodes, edges)

def longest_path(adj_list):
    def dfs(curr, parent):
        longest = 0
        for nbr in adj_list[curr]:
            if nbr != parent:
                longest = max(longest, 1 + dfs(nbr, curr))
        return longest
    result = 0
    for start in adj_list:
        longest = dfs(start, -1)
        print(start, longest)
        result = max(result, longest)
    return result

print(longest_path(adj_list))


