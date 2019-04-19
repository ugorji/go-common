package tree 

import (
	"reflect"
	"testing"
	"bytes"
	"fmt"
)

func TestNode(t *testing.T) {
	vals := []interface{}{1, 2, -2, 21, -2, 211, 212, -2, 2121, 2122, 2123, -1, -1, 22, 23, -1, 3}
	t.Logf("Source:       %#v", vals)
	codec := &Codec{Desc: -2, Asc: -1}
	root := &Node { }
	codec.DecodeChildren(root, vals)
	vals2 := codec.EncodeChildren(root, nil)
	t.Logf("After Encode: %#v", vals2)
	if !reflect.DeepEqual(vals, vals2) { 
		t.Errorf("Initial slice: %v, different from Decoded slice: %v", vals, vals2) 
	}
}

func TestWalk(t *testing.T) {
	ctr := 0
	fn := func(n *Node, evt Event) (err error) {
		switch evt {
		case VisitEvent:
			if ctr = ctr + 1; ctr%2 == 0 {
				n.Value = fmt.Sprintf("%v-%v", n.Value, 999)
			}
		}
		return
	}
	vals := []interface{}{1, 2, -2, 21, -2, 211, 212, -2, 2121, 2122, 2123, -1, -1, 22, 23, -1, 3}
	codec := &Codec{Desc: -2, Asc: -1}
	root := &Node { }
	codec.DecodeChildren(root, vals)
	root2 := root.Copy()
	root2.Walk(true, false, fn)
	indentString := "-- "
	w := new(bytes.Buffer)
	w.WriteString("\n")
	w.WriteString("====== (breadthfirst, forward) root: \n")
	root.Write(false, false, w, indentString)
	w.WriteString("====== (depthfirst, forward) root: \n")
	root.Write(true, false, w, indentString)
	w.WriteString("====== (breadthfirst, reverse) root: \n")
	root.Write(false, true, w, indentString)
	w.WriteString("====== (breadthfirst, reverse) root2: \n")
	root2.Write(false, true, w, indentString)
	w.WriteString("====== \n")
	t.Log(string(w.Bytes()))
	//println("")
	_, _ = fn, root2
	//tree.Walk(root, true, 
}
