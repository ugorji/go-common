/*
 The XXX types and corresponding functions supports a tree structure (node with child nodes).
 
 You can 
   - represent a tree as a 1-dimensional slice
   - walk a tree
   - write tree out to an input stream as text.

 This code has now been updated to remove reference to parents, and prevent 
 the cyclic structure. This allows us to encode a Tree using gob.

 REPRESENTATION AS SLICE
 
 A tree can be representated as a 1-dimensional slice.
 
 Given a tree like:
   ROOT
     a1
     a2
       b1
          c1
          c2
          c3
            d1  
            d2 
            d3
       b2
       b3
     a3
 And 
    DESC, RET
 Encode to an an array like:
    a1, a2, DESC, b1, DESC, c1, c2, DESC, d1, d2, d3, RET, RET, b2, b3, RET, a3
 Decode same array to a tree as above.

 WALKING

 You can walk a slice or write it out to an input stream as text.

 During a walk, all the walk events are made available to the walk function.

 Walk Algorithm:
   Perform EnterEvent operation
   if breadth first, Perform VisitEvent operation
   for i=0 to n-1 do
     Walk child[i], if present
     Perform BackUpEvent operation
   if depth first, Perform VisitEvent operation

*/
package tree

/*
The code is also written so that it can easily be re-packaged for different types.

Shared code is placed in this file, so that any_tree.go is specific interface{} type,
and we can easily use sed to generate implementation for other specific types.

Implementations for specific types (e.g. int64) are made by copying any_tree.go and 
doing multiple passes of in-place sed on that copy. 

** Make sure you run this each time any_tree.go is updated. **

  cp any_tree.go int64_tree.go
  sed -e 's+interface{}+int64+g' -i int64_tree.go
  sed -e 's+DefDesc = new(struct{})+Int64DefDesc = int64(-2)+g' -i int64_tree.go
  sed -e 's+DefAsc  = new(struct{})+Int64DefAsc  = int64(-1)+g' -i int64_tree.go
  sed -e 's+Def+Int64Def+g' -i int64_tree.go
  sed -e 's+Walk+Int64Walk+g' -i int64_tree.go
  sed -e 's+Node+Int64Node+g' -i int64_tree.go
  sed -e 's+Codec+Int64Codec+g' -i int64_tree.go

*/

import (
	"errors"
)

const (
  //Event done at start of walking each node
  EnterEvent  Event = iota + 1
  //Event done after walking each child node (return to parent)
  BackUpEvent
  //Event done to visit node
  VisitEvent 
)

var (
	//StopDescent is returned as an error during a Walk to signify 
	//that the Walk should not proceed below this node for this edge.
	StopDescent = errors.New("Stop Descent")
)


//A Event is passed to each invocation of WalkFunc.
type Event int

