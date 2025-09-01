package main

//TODO: Think of how to implement iterativeFindNode in the most efficient way as the next step.
import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"sync"
)

const (
	IDBits  = 160
	IDBytes = IDBits / 8 //20
	K       = 20
	Alpha   = 3
)

// ID is a 160-bit identifier
// JSON-encoded as hex string

type ID [IDBytes]byte

func (id ID) String() string { return hex.EncodeToString(id[:]) }

func (id *ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(id[:]))
}

func (id *ID) unmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != IDBytes {
		return fmt.Errorf("invalid id hex")
	}
	copy(id[:], raw)
	return nil
}

func NewRandomID() ID {
	var id ID
	rand.Read(id[:])
	return id
}

func HashKey(key string) ID {
	var id ID
	sum := sha1.Sum([]byte(key))
	copy(id[:], sum[:])
	return id
}

func xor(a, b ID) (out ID) {
	for i := range IDBytes {
		out[i] = a[i] ^ b[i]
	}
	return
}

// distanceLess reports whether dist(a,x) < dist(b,x) for a common target x by
// comparing XOR distances lexicographically (big-endian).
func distanceLess(a, b, target ID) bool {
	da := xor(a, target)
	db := xor(b, target)

	for i := range IDBytes {
		if da[i] == db[i] {
			continue
		}
		return da[i] < db[i]
	}
	return false
}

// bucket index is floor(log2(distance)) in [0..159]; if distance==0 return 0
func bucketIndex(self, other ID) int {
	d := xor(self, other)
	for i := range IDBytes {
		if d[i] == 0 {
			continue
		}
		for bit := 7; bit >= 0; bit-- {
			if (d[i]>>uint(bit))&1 == 1 {
				return (IDBytes-1-i)*8 + (7 - bit)
			}
		}
	}
	return 0
}

// Contact represents a node in the network.

type Contact struct {
	ID   ID     `json:"id"`
	Addr string `json:"addr"` // "ip:port"
}

// K-bucket with naive LRU management.

type bucket struct {
	mu       sync.Mutex
	entries  []Contact // most-recent at front
	capacity int
}

func newBucket(cap int) *bucket { return &bucket{capacity: cap} }

func (b *bucket) touch(c Contact) (evicted *Contact) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, e := range b.entries {
		if e.ID == c.ID {
			b.entries[i] = c
			if i != 0 {
				copy(b.entries[1:i+1], b.entries[0:i])
				b.entries[0] = c
			}
			return nil
		}
	}

	//insert at front
	b.entries = append([]Contact{c}, b.entries...)
	if len(b.entries) > b.capacity {
		victim := b.entries[len(b.entries)-1]
		b.entries = b.entries[:len(b.entries)-1]
		return &victim
	}
	return nil
}

func (b *bucket) list() []Contact {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Contact, len(b.entries))
	copy(out, b.entries)
	return out
}

// Routing table is 160 buckets
type RoutingTable struct {
	self    ID
	buckets [IDBits]*bucket
}

func newRoutingTable(self ID) *RoutingTable {
	rt := &RoutingTable{self: self}
	for i := range IDBits {
		rt.buckets[i] = newBucket(K)
	}
	return rt
}

func (rt *RoutingTable) Update(c Contact) {
	if c.ID == rt.self || c.Addr == "" {
		return
	}
	idx := bucketIndex(rt.self, c.ID)
	rt.buckets[idx].touch(c)
}

func (rt *RoutingTable) FindClosest(target ID, count int) []Contact {
	var candidates []Contact

	for i := range IDBits {
		candidates = append(candidates, rt.buckets[i].list()...)
	}
	//unique by ID
	seen := make(map[ID]bool)
	uniq := candidates[:0]

	for _, c := range candidates {
		if !seen[c.ID] {
			seen[c.ID] = true
			uniq = append(uniq, c)
		}
	}
	// sort by distance to target
	sort.Slice(uniq, func(i, j int) bool { return distanceLess(uniq[i].ID, uniq[j].ID, target) })
	if len(uniq) > count {
		uniq = uniq[:count]
	}
	return uniq
}

// ------------------------ RPC layer over UDP (JSON) ------------------------
type Message struct {
	Type      string          `json:"type"`
	RequestID string          `json:"rid"`
	From      Contact         `json:"from"`
	Payload   json.RawMessage `json:"payload"`
}

type Ping struct{}

type Pong struct{}

type FindNodeReq struct {
	Target ID `json:"target"`
}

type FindNodeResp struct {
	Nodes []Contact `json:"nodes"`
}

type StoreReq struct {
	Key   string `json:"key"`
	Value []byte `json:"value"`
}

type StoreResp struct {
	OK bool `json:"ok"`
}

type FindValueReq struct {
	Key string `json:"key"`
}

type FindValueResp struct {
	Value []byte    `json:"value"`
	Nodes []Contact `json:"nodes"`
}

// ---------------- Node ----------------

type Node struct {
	ID      ID
	Addr    string
	rt      *RoutingTable
	store   map[string][]byte
	mu      sync.RWMutex
	conn    *net.UDPConn
	pending map[string]chan Message
	pMu     sync.Mutex
	alpha   int
	k       int
}

func NewNode(listenAddr string, id ID) (*Node, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	n := &Node{
		ID:      id,
		Addr:    conn.LocalAddr().String(),
		rt:      newRoutingTable(id),
		store:   make(map[string][]byte),
		conn:    conn,
		pending: make(map[string]chan Message),
		alpha:   Alpha,
		k:       K,
	}
	go n.serve()
	return n, nil
}

func (n *Node) Close() { _ = n.conn.Close() }

func (n *Node) serve() {
	buf := make([]byte, 64*1024)
	for {
		nRead, raddr, err := n.conn.ReadFromUDP(buf)
		if err != nil {
			return //Closed
		}
		var msg Message
		if err := json.Unmarshal(buf[:nRead], &msg); err != nil {
			continue
		}

		//update routing table with sender
		n.rt.Update(msg.From)
		//Check if this is a response were waiting on
		n.pMu.Lock()
		ch, ok := n.pending[msg.RequestID]
		n.pMu.Unlock()
		if ok {
			//deliver response
			select {
			case ch <- msg:
			default:
			}
			continue
		}
		//Otherwise its a request -> handle
		go n.handleRequest(msg, raddr)
	}
}

func (n *Node) handleRequest(msg Message, raddr *net.UDPAddr) {
	reply := func(payload any) {
		b, _ := json.Marshal(payload)
		resp := Message{Type: msg.Type + ":resp", RequestID: msg.RequestID, From: Contact{ID: n.ID, Addr: n.Addr}, Payload: b}
		data, _ := json.Marshal(resp)
		n.conn.WriteToUDP(data, raddr)
	}

	switch msg.Type {
	case "ping":
		reply(Pong{})
	case "find_node":
		var req FindNodeReq
		_ = json.Unmarshal(msg.Payload, &req)
		nodes := n.rt.FindClosest(req.Target, n.k)
		reply(FindNodeResp{Nodes: nodes})
	case "store":
		var req StoreReq
		_ = json.Unmarshal(msg.Payload, &req)
		n.mu.Lock()
		n.store[req.Key] = req.Value
		n.mu.Unlock()
		reply(StoreResp{OK: true})
	case "find_value":
		var req FindValueReq
		_ = json.Unmarshal(msg.Payload, &req)
		n.mu.Lock()
		val, ok := n.store[req.Key]
		n.mu.Unlock()
		if ok {
			reply(FindValueResp{Value: val})
			return
		}
		nodes := n.rt.FindClosest(HashKey(req.Key), n.k)
		reply(FindValueResp{Nodes: nodes})
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (n *Node) rpc(ctx context.Context, addr string, typ string, payload any, expect string) (Message, error) {
	var zero Message
	b, err := json.Marshal(payload)
	if err != nil {
		return zero, err
	}
	msg := Message{Type: typ, RequestID: randHex(8), From: Contact{ID: n.ID, Addr: n.Addr}, Payload: b}
	data, _ := json.Marshal(payload)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return zero, err
	}
	ch := make(chan Message, 1)
	n.pMu.Lock()
	n.pending[msg.RequestID] = ch
	n.pMu.Unlock()
	defer func() { n.pMu.Lock(); delete(n.pending, msg.RequestID); n.pMu.Unlock() }()

	if _, err = n.conn.WriteToUDP(data, udpAddr); err != nil {
		return zero, err
	}
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case resp := <-ch:
		if resp.Type != expect || resp.RequestID != msg.RequestID {
			return zero, fmt.Errorf("unexpected response")
		}
		return resp, nil
	}
}

func (n *Node) ping(ctx context.Context, addr string) error {
	_, err := n.rpc(ctx, addr, "ping", Ping{}, "ping:resp")
	return err
}
