# Horriya
**Horriya** is an attempt at making a **dead simple decentralized social media** where people can join the network by running the app and joining the network of nodes. It needs to be **pure p2p** while being as **lightweight as possible** in order to be compatible with many devices. 

# The goal of it is to have at the end: 
- A pure p2p network of nodes communicating with each other 
- The nodes store the data as well. 
- Every post consists of text and optionally a link to an image stored on ipfs
- posts shouldn't be lost when one goes offline, and a single post should be always available among many several nodes to ensure it is never lost when many go offline. The idea being that a post shouldn't only be available locally and should always be found once a person reconnects even after several years 
- Users have the ability to make posts and to select the mode they are in. The modes are "discovery" or "following". So the users are also able to follow each other. The "discovery" mode allows users to find new posts from random other people. And a post's reach can only be determined by one's "influence". Finally, they can upvote or downvote a post. 
- Influence is at the core of horriya. It is the metric that determines your visibility, and in the future will also determine the importance of your input. 
Influence is just like karma on reddit. But instead of being steady over time, it decreases by a point everyday. And if a person has a lot of it, then their influence decreases exponentially faster. 
- Posts should be verified on one's client to ensure that fake posts provided by other nodes are verified with the signature attached to ensure only valid and authentic posts are displayed and the rest are filtered. 
- The influence mechanism should handle bots automatically, as these will get downvoted badly. 
- A user cannot have negative influence. Lowest being 0. 
- As the goal is to reach a fully p2p network, there should be a consensus to verify and store new posts. And to slash bad actors among nodes. For example those that are spreading false posts that have a wrong signature, or those that are censoring posts and decide not to propagate them. Therefore, there is a cap of influence that is issued on horriya every 24 hours on the digital distributed clock. The influence should be proportionate to the number of users and number of posts validated by each node.

# The things that I still need to determine TECH WISE AND/OR Understand**

- Which discovery mechanism I should implement (Discv5 is the only one I know)

- How content is sent to nodes (Gossip protocol maybe ?)

- How the posts are stored (& shared). (Maybe DHT ?)

- How the consensus mechanism works to ensure balance among the system and mitigate attacks such as bots AND/OR vulnerability to low cost 

- How to mitigate the risks of censorship if most nodes are owned by a single party 

- How to ensure the privacy of indivuduals so that they have true freedom of speech with no censorship or fear while remaining anonymous (Maybe follow the onion routing mechanism ?)

- Finally, what formula / mechanism to use to form the influence consensus & to share the influence among users (maybe a tangent to how bitcoin works but with different mechanisms. Use it only as inspiration) **Maybe use of blockchain technology for keeping record of influence ?**

- Maybe no need of influence at all ? 