
package tree 

import (
	"io"
	"errors"
	"fmt"
)


var (
	DefDesc = new(struct{})
	DefAsc  = new(struct{})
)

//Function called for each event during a TreeWalk.
//Return StopDescent if you don't want to walk down a node further.
type WalkFunc func(n *Node, evt Event) (err error)

type Node struct {
	//don't keep a reference to the parent.	
	Value interface{}
	Children []*Node
}

//A Codec is used to encode or decode a Node into/from a slice.
//Slices are easier to store and manage, especially in some app environments e.g. Google App Engine.
type Codec struct {
	Desc interface{}
	Asc  interface{}
}

func NewCodec() Codec {
	return Codec{ Desc: DefDesc, Asc: DefAsc }
}

//Decodes the children of Node n from the slice.
//Note that the "root" ie n is not reflected in the slice.
func (c Codec) DecodeChildren(n *Node, vals []interface{}) {
	stack := make([]*Node, 0, 4)
	stack = append(stack, n)
	justDesc := false
	nx := n
	for i := 0; i < len(vals); i++ {
		switch vals[i] {
		case c.Desc:
			justDesc = true
		case c.Asc:
			if justDesc { 
				justDesc = false
			} else {
				stack = stack[:len(stack)-1]
			}
		default:
			if justDesc {
				stack = append(stack, nx)
				justDesc = false
			}
			nx = &Node { Value: vals[i] }
			ny := stack[len(stack)-1]
			ny.Children = append(ny.Children, nx)
		}
	}
}

//Encodes the children of Node n into the slice .
//Note that the "root" ie n is not reflected in the slice.
//It's okay to pass a nil slice for starters.
func (c Codec) EncodeChildren(n *Node, vals []interface{}) ([]interface{}) {
	for _, n2 := range n.Children {
		vals = append(vals, n2.Value)
		if len(n2.Children) > 0 { 
			vals = append(vals, c.Desc) 
			vals = c.EncodeChildren(n2, vals)
			vals = append(vals, c.Asc) 
		}
	}
	return vals
}

//Copy returns a deep copy of Node n.
//This just means that a copy of the node and copies of the children are returned. 
//However, the "Value" in each node stays the same.
func (n *Node) Copy() (n2 *Node) {
	n2 = &Node{Value: n.Value}
	if len(n.Children) == 0 {
		return
	}
	n2.Children = make([]*Node, len(n.Children))
	for i, nx := range n.Children {
		if nx != nil {
			n2.Children[i] = nx.Copy()
		}
	}
	return
}

func (n0 *Node) fnc(evt0 Event, fn WalkFunc, xerr *error) (shouldIReturn bool) {
	var err error
	if err = fn(n0, evt0); err != nil {
		if err == StopDescent {
			err = nil
		} 
		shouldIReturn = true
	}
	*xerr = err
	return
}

func (n *Node) dowalk(depthFirst bool, reverse bool, fn WalkFunc) (err error) {
	xerr := &err
	if n.fnc(EnterEvent, fn, xerr) { return }
	if !depthFirst {
		if n.fnc(VisitEvent, fn, xerr) { return }
	}
	if len(n.Children) > 0 {
		for i, nx := range n.Children {
			if reverse {
				nx = n.Children[len(n.Children) - 1 - i]
			}
			if err = nx.dowalk(depthFirst, reverse, fn); err != nil {
				return
			}
			if n.fnc(BackUpEvent, fn, xerr) { return }
		}
	}
	if depthFirst {
		if n.fnc(VisitEvent, fn, xerr) { return }
	}
	return
}

//Walk Node n, depth first or breadth first, and call fn
//for every event during the Walk. 
func (n *Node) Walk(depthFirst bool, reverse bool, fn WalkFunc) (err error) {
	err = n.dowalk(depthFirst, reverse, fn)
	return
}

//Write a Node into a Writer, using the indent string given to signify indents.
func (n *Node) Write(depthFirst bool, reverse bool, w io.Writer, indentStr string) error {
	stack := make([]*Node, 0, 4)
	indent := 0
	fn := func(n *Node, evt Event) (err error) {
		switch evt {
		case EnterEvent:
			stack = append(stack, n)
			indent++
		case VisitEvent:
			for i := 0; i < indent; i++ {
				io.WriteString(w, indentStr)
			}
			io.WriteString(w, fmt.Sprintf("%v\n", n.Value))
			//fallthrough
		case BackUpEvent:
			stack = stack[:len(stack)-1]
			indent--
		default:
			err = errors.New(fmt.Sprintf("Unknown event: %v", evt))
		}
		return 
	}	
	_ = stack
	return n.Walk(depthFirst, reverse, fn)
}

