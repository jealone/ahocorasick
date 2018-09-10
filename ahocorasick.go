// ahocorasick.go: implementation of the Aho-Corasick string matching
// algorithm. Actually implemented as matching against []byte rather
// than the Go string type. Throughout this code []byte is referred to
// as a blice.
//
// http://en.wikipedia.org/wiki/Aho%E2%80%93Corasick_string_matching_algorithm
//
// Copyright (c) 2013 CloudFlare, Inc.

package ahocorasick

import (
	"container/list"
	"fmt"
	"bytes"
)

// A node in the trie structure used to implement Aho-Corasick
type node struct {
	root bool // true if this is the root

	b []byte // The blice at this node

	output bool // True means this node represents a blice that should
	// be output when matching
	index int // index into original dictionary if output is true

	//counter int // Set to the value of the Matcher.counter when a
	// match is output to prevent duplicate output

	// The use of fixed size arrays is space-inefficient but fast for
	// lookups.
	object string

	child [256]*node // A non-nil entry in this array means that the
	// index represents a byte value which can be
	// appended to the current node. Blices in the
	// trie are built up byte by byte through these
	// child node pointers.

	fails [256]*node // Where to fail to (by following the fail
	// pointers) for each possible byte

	suffix *node // Pointer to the longest possible strict suffix of
	// this node

	fail *node // Pointer to the next node which is in the dictionary
	// which can be reached from here following suffixes. Called fail
	// because it is used to fallback in the trie when a match fails.
}

type value struct {
	blice []byte
	object string
}

// Matcher is returned by NewMatcher and contains a list of blices to
// match against
type Matcher struct {
	//counter int // Counts the number of matches done, and is used to
	// prevent output of multiple matches of the same string
	trie []node // preallocated block of memory containing all the
	// nodes
	extent int   // offset into trie that is currently free
	root   *node // Points to trie[0]
}

// finndBlice looks for a blice in the trie starting from the root and
// returns a pointer to the node representing the end of the blice. If
// the blice is not found it returns nil.
func (m *Matcher) findBlice(b []byte) *node {
	n := &m.trie[0]

	for n != nil && len(b) > 0 {
		n = n.child[int(b[0])]
		b = b[1:]
	}

	return n
}

// getFreeNode: gets a free node structure from the Matcher's trie
// pool and updates the extent to point to the next free node.
func (m *Matcher) getFreeNode() *node {
	m.extent += 1

	if m.extent == 1 {
		m.root = &m.trie[0]
		m.root.root = true
	}

	return &m.trie[m.extent-1]
}

// buildTrie builds the fundamental trie structure from a set of
// blices.
//func (m *Matcher) buildTrie(dictionary [][]byte) {
func (m *Matcher) buildTrie(dictionary []*value) {

	// Work out the maximum size for the trie (all dictionary entries
	// are distinct plus the root). This is used to preallocate memory
	// for it.

	max := 1
	for _, blice := range dictionary {
		max += len(blice.blice)
	}
	m.trie = make([]node, max)

	// Calling this an ignoring its argument simply allocated
	// m.trie[0] which will be the root element

	m.getFreeNode()

	// This loop builds the nodes in the trie by following through
	// each dictionary entry building the children pointers.

	for i, blice := range dictionary {
		n := m.root
		var path []byte
		for _, b := range blice.blice {
			path = append(path, b)

			c := n.child[int(b)]

			if c == nil {
				c = m.getFreeNode()
				n.child[int(b)] = c
				c.b = make([]byte, len(path))
				copy(c.b, path)

				// Nodes directly under the root node will have the
				// root as their fail point as there are no suffixes
				// possible.

				if len(path) == 1 {
					c.fail = m.root
				}

				c.suffix = m.root
			}

			n = c
		}

		// The last value of n points to the node representing a
		// dictionary entry
		n.object = blice.object
		n.output = true
		n.index = i
	}

	l := new(list.List)
	l.PushBack(m.root)

	for l.Len() > 0 {
		n := l.Remove(l.Front()).(*node)

		for i := 0; i < 256; i++ {
			c := n.child[i]
			if c != nil {
				l.PushBack(c)

				for j := 1; j < len(c.b); j++ {
					c.fail = m.findBlice(c.b[j:])
					if c.fail != nil {
						break
					}
				}

				if c.fail == nil {
					c.fail = m.root
				}

				for j := 1; j < len(c.b); j++ {
					s := m.findBlice(c.b[j:])
					if s != nil && s.output {
						c.suffix = s
						break
					}
				}
			}
		}
	}

	for i := 0; i < m.extent; i++ {
		for c := 0; c < 256; c++ {
			n := &m.trie[i]
			for n.child[c] == nil && !n.root {
				n = n.fail
			}

			m.trie[i].fails[c] = n
		}
	}

	m.trie = m.trie[:m.extent]
}

// NewMatcher creates a new Matcher used to match against a set of
// blices
func NewMatcher(dictionary map[string]string) *Matcher {
//func NewMatcher(dictionary []map[string]interface{}) *Matcher {
	m := new(Matcher)

	//dict := make([]*value, 0, len(dictionary))
	var dict []*value

	for k, v := range dictionary {
		dict = append(dict, &value{blice:[]byte(k), object:v})
	}

	m.buildTrie(dict)

	return m
}

//NewStringMatcher creates a new Matcher used to match against a set
//of strings (this is a helper to make initialization easy)
//func NewStringMatcher(dictionary []string) *Matcher {
//	m := new(Matcher)
//
//	var d []*value
//	for _, s := range dictionary {
//		seg := strings.SplitN(s, "|", 2)
//		if len(seg) < 2 {
//			continue
//		}
//		fmt.Println(seg)
//		d = append(d, &value{blice:[]byte(seg[0]), object:seg[1]})
//	}
//
//	m.buildTrie(d)
//
//	return m
//}

// Match searches in for blices and returns all the blices found as
// indexes into the original dictionary
//func (m *Matcher) Match(in []byte) []int {
//	m.counter += 1
//	var hits []int
//
//	n := m.root
//
//	for _, b := range in {
//		c := int(b)
//
//		if !n.root && n.child[c] == nil {
//			n = n.fails[c]
//		}
//
//		if n.child[c] != nil {
//			f := n.child[c]
//			n = f
//
//			if f.output && f.counter != m.counter {
//				hits = append(hits, f.index)
//				f.counter = m.counter
//			}
//
//			for !f.suffix.root {
//				f = f.suffix
//				if f.counter != m.counter {
//					hits = append(hits, f.index)
//					f.counter = m.counter
//				} else {
//
//					// There's no point working our way up the
//					// suffixes if it's been done before for this call
//					// to Match. The matches are already in hits.
//
//					break
//				}
//			}
//		}
//	}
//
//	return hits
//}

type matchList struct {
	root *matchNode
	last *matchNode
}

func (m *matchList) add(s int,e int, key string, obj string) error {
	//fmt.Printf("start:%d, end:%d\n", s, e)
	if s >= e {
		return fmt.Errorf("start must less than end")
	}
	if  m.root == nil || m.last == nil {
		node := new(matchNode)
		node.start = s
		node.end = e
		node.key = key
		node.object = obj
		m.root = node
		m.last = node
		return nil
	}

	//fmt.Printf("=======last end:%d, start:%d=======\n", m.last.end, s)
	if m.last.end <= s {
		node := new(matchNode)
		node.start = s
		node.end = e
		node.key = key
		node.object = obj
		m.last.next = node
		node.prior = m.last
		m.last = node
		//fmt.Println(node)
	} else if m.last.end - m.last.end < e - s {
		m.last.key = key
		m.last.object = obj
		m.last.start = s
		m.last.end = e
	}
	return nil
}

type matchNode struct {
	key string
	object string
	start int
	end int
	prior *matchNode
	next *matchNode
}

func (m *Matcher) Replace(in []byte) []byte {
	//fmt.Println(string(in))
	match := &matchList{}
	var hits []map[string]int

	n := m.root

	for pos, b := range in {
		c := int(b)

		if !n.root && n.child[c] == nil {
			n = n.fails[c]
		}

		if n.child[c] != nil {
			f := n.child[c]
			n = f

			if f.output {
				match.add(pos+1-len(f.b), pos+1, string(f.b), f.object)
				//fmt.Println(f.object)
				//hits = append(hits, map[string]int{"pos":pos, "index":f.index, "len":len(f.b)})
				hits = append(hits, map[string]int{"start":pos+1-len(f.b), "index":f.index, "end":pos+1})
			}

			for !f.suffix.root {
				f = f.suffix
				//match.add(pos+1-len(f.b), pos+1)
				match.add(pos+1-len(f.b), pos+1, string(f.b), f.object)
				//hits = append(hits, map[string]int{"pos":pos, "index":f.index, "len":len(f.b)})
				hits = append(hits, map[string]int{"start":pos+1-len(f.b), "index":f.index, "end":pos+1})
			}
		}
	}
	//node := match.root
	var b bytes.Buffer
	index := 0
	for node := match.root; node != nil; {
		if index == node.start {
			b.WriteString(node.object)
			index = node.end
		} else if index < node.start {
			b.Write(in[index:node.start])
			b.WriteString(node.object)
			index = node.end
		}
		//b.Write()
		//fmt.Printf("start:%d, end:%d\n", node.start, node.end)
		//fmt.Printf("key:%s, obj:%s\n", node.key, node.object)
		node = node.next
	}
	if index != len(in) {
		b.Write(in[index:])
	}

	return b.Bytes()
}

func (m *Matcher) Match(in []byte) []map[string]int {
	fmt.Println(string(in))
	var hits []map[string]int

	n := m.root

	for pos, b := range in {
		c := int(b)

		if !n.root && n.child[c] == nil {
			n = n.fails[c]
		}

		if n.child[c] != nil {
			f := n.child[c]
			n = f

			if f.output {
				//hits = append(hits, map[string]int{"pos":pos, "index":f.index, "len":len(f.b)})
				hits = append(hits, map[string]int{"start":pos+1-len(f.b), "index":f.index, "end":pos+1})
			}

			for !f.suffix.root {
				f = f.suffix
				//hits = append(hits, map[string]int{"pos":pos, "index":f.index, "len":len(f.b)})
				hits = append(hits, map[string]int{"start":pos+1-len(f.b), "index":f.index, "end":pos+1})
			}
		}
	}

	return hits
}
