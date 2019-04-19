
package tree 

import (
	"io"
	"errors"
	"fmt"
)


var (
	Int64DefDesc = int64(-2)
	Int64DefAsc  = int64(-1)
)

//Function called for each event during a Int64Walk.
//Return StopDescent if you don't want to walk down a node further.
type Int64WalkFunc func(n *Int64Node, evt Event) (err error)

type Int64Node struct {
	//don't keep a reference to the parent.	
	Value int64
	Children []*Int64Node
}

//A Int64Codec is used to encode or decode a Int64Node into/from a slice.
//Slices are easier to store and manage, especially in some app environments e.g. Google App Engine.
type Int64Codec struct {
	Desc int64
	Asc  int64
}

func NewInt64Codec() Int64Codec {
	return Int64Codec{ Desc: Int64DefDesc, Asc: Int64DefAsc }
}

//Decodes the children of Int64Node n from the slice.
//Note that the "root" ie n is not reflected in the slice.
func (c Int64Codec) DecodeChildren(n *Int64Node, vals []int64) {
	stack := make([]*Int64Node, 0, 4)
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
			nx = &Int64Node { Value: vals[i] }
			ny := stack[len(stack)-1]
			ny.Children = append(ny.Children, nx)
		}
	}
}

//Encodes the children of Int64Node n into the slice .
//Note that the "root" ie n is not reflected in the slice.
//It's okay to pass a nil slice for starters.
func (c Int64Codec) EncodeChildren(n *Int64Node, vals []int64) ([]int64) {
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

//Int64Copy returns a deep copy of Int64Node n.
//This just means that a copy of the node and copies of the children are returned. 
//However, the "Value" in each node stays the same.
func (n *Int64Node) Copy() (n2 *Int64Node) {
	n2 = &Int64Node{Value: n.Value}
	if len(n.Children) == 0 {
		return
	}
	n2.Children = make([]*Int64Node, len(n.Children))
	for i, nx := range n.Children {
		if nx != nil {
			n2.Children[i] = nx.Copy()
		}
	}
	return
}

func (n0 *Int64Node) fnc(evt0 Event, fn Int64WalkFunc, xerr *error) (shouldIReturn bool) {
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

func (n *Int64Node) dowalk(depthFirst bool, reverse bool, fn Int64WalkFunc) (err error) {
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

//Walk Int64Node n, depth first or breadth first, and call fn
//for every event during the Int64Walk. 
func (n *Int64Node) Walk(depthFirst bool, reverse bool, fn Int64WalkFunc) (err error) {
	err = n.dowalk(depthFirst, reverse, fn)
	return
}

//Write a Int64Node into a Writer, using the indent string given to signify indents.
func (n *Int64Node) Write(depthFirst bool, reverse bool, w io.Writer, indentStr string) error {
	stack := make([]*Int64Node, 0, 4)
	indent := 0
	fn := func(n *Int64Node, evt Event) (err error) {
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

