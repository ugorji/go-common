package tree

import (
	"errors"
)

const (
	//Event done at start of walking each node
	EnterEvent Event = iota + 1
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
