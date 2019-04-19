package feed

import (
	"time"
)

type textType uint8

const (
	TextPlain textType = iota + 1
	TextHtml
	TextXHtml
)

type Feed struct {
	Title    string
	Link     string
	SelfLink string
	PubDate  time.Time
	LastMod  time.Time
	Entries  []*Entry
	Author   string
}

type Entry struct {
	Title    string
	Link     string
	Desc     string
	DescType textType
	PubDate  time.Time
	LastMod  time.Time
	Author   string
}
