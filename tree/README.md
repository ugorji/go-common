# go-common/tree

This repository contains the `go-common/tree` library (or command).

To install:

```
go get github.com/ugorji/go-common/tree
```

# Package Documentation


Package tree implements traversal of tree structures (parent node with child
nodes).

The variant types and corresponding functions supports a tree structure
(node with child nodes).

You can:

  - represent a tree as a 1-dimensional slice
  - walk a tree
  - write tree out to an input stream as text.

This code has now been updated to remove reference to parents, and prevent
the cyclic structure. This allows us to encode a Tree using gob.


## REPRESENTATION AS SLICE

A tree can be representated as a 1-dimensional slice.

Given a tree like:

```
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
```

## And

```
    DESC, RET
```

Encode to an an array like:

```
    a1, a2, DESC, b1, DESC, c1, c2, DESC, d1, d2, d3, RET, RET, b2, b3, RET, a3
```

Decode same array to a tree as above.


## WALKING

You can walk a slice or write it out to an input stream as text.

During a walk, all the walk events are made available to the walk function.

## Walk Algorithm

```
    Perform EnterEvent operation
    if breadth first, Perform VisitEvent operation
    for i=0 to n-1 do
      Walk child[i], if present
      Perform BackUpEvent operation
    if depth first, Perform VisitEvent operation
```


## Code Generation

any_tree.go is the generic version of the tree.

We auto-generate the specific versions for an int64 tree and a uint64 tree.

You can update gen.sh to auto-generate for other types.

Run `go generate` (or gen.sh) in this directory to re-generate them.

```
    Make sure you run this each time any_tree.go is updated.
```

## Exported Package API

```go
var DefDesc = new(struct{}) ...
var Int64DefDesc = int64(math.MinInt64 + 1) ...
var Uint64DefDesc = uint64(math.MaxUint64 - 1) ...
var StopDescent = errors.New("Stop Descent")
type Codec struct{ ... }
    func NewCodec() Codec
type Event int
    const EnterEvent Event = iota + 1 ...
type Int64Codec struct{ ... }
    func NewInt64Codec() Int64Codec
type Int64Node struct{ ... }
type Int64WalkFunc func(n *Int64Node, evt Event) (err error)
type Node struct{ ... }
type Uint64Codec struct{ ... }
    func NewUint64Codec() Uint64Codec
type Uint64Node struct{ ... }
type Uint64WalkFunc func(n *Uint64Node, evt Event) (err error)
type WalkFunc func(n *Node, evt Event) (err error)
```
