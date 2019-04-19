package feed

import (
	"encoding/xml"
	"html"
	"io"
	"net/url"
	"time"
)

// NOTES:
// Use pointers for fields which are optional. Also specify omitempty where possible.
//
// Feed should have self, and must have alternate link.
// Feed must have author, so it stays optional for entries. default to "-" if specified.
// Entry must have alternate link.
//
// Order of fields is important for entry/feedAtom, since xml marshals them in order.

type authorAtom struct {
	XMLName xml.Name `xml:"author"`
	Name    string   `xml:"name,omitempty"`
}

type summaryAtom struct {
	XMLName xml.Name `xml:"summary"`
	S       string   `xml:",chardata"`
	Type    string   `xml:"type,attr"`
}

type linkAtom struct {
	XMLName xml.Name `xml:"link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr"`
	Type    string   `xml:"type,attr"`
}

type entryAtom struct {
	XMLName   xml.Name `xml:"entry"`
	Id        string   `xml:"id"`
	Title     string   `xml:"title"`
	Author    *authorAtom
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
	Link      linkAtom
	Summary   *summaryAtom
}

type feedAtom struct {
	XMLName   xml.Name    `xml:"feed"`
	Ns        string      `xml:"xmlns,attr"`
	Id        string      `xml:"id"`
	Title     string      `xml:"title"`
	Author    *authorAtom // pointer so it can be nil
	Published string      `xml:"published"`
	Updated   string      `xml:"updated"`
	Links     []linkAtom
	Entries   []*entryAtom
}

func toAtomEntry(e *Entry) *entryAtom {
	idDate := e.PubDate.Format("2006-01-02")
	id := "tag:" + e.Link + "," + idDate + ":/invalid.html"
	if url, err := url.Parse(e.Link); err == nil {
		id = "tag:" + url.Host + "," + idDate + ":" + url.Path
	}
	xe := entryAtom{
		Title:     e.Title,
		Link:      linkAtom{Href: e.Link, Rel: "alternate", Type: "text/html"},
		Id:        id,
		Published: e.PubDate.Format(time.RFC3339),
		Updated:   e.LastMod.Format(time.RFC3339),
	}
	if e.Desc != "" {
		switch e.DescType {
		case TextPlain:
			xe.Summary = &summaryAtom{S: e.Desc, Type: "text"}
		case TextHtml:
			xe.Summary = &summaryAtom{S: html.EscapeString(e.Desc), Type: "html"}
		case TextXHtml:
			xe.Summary = &summaryAtom{S: e.Desc, Type: "xhtml"}
		}
	}
	if e.Author != "" {
		xe.Author = &authorAtom{Name: e.Author}
	}
	return &xe
}

func toAtomFeed(f *Feed) *feedAtom {
	xf := feedAtom{
		Ns:        "http://www.w3.org/2005/Atom",
		Title:     f.Title,
		Id:        f.Link,
		Published: f.PubDate.Format(time.RFC3339),
		Updated:   f.LastMod.Format(time.RFC3339),
	}
	xf.Links = append(xf.Links, linkAtom{Href: f.Link, Rel: "alternate", Type: "text/html"})
	if f.SelfLink != "" {
		xf.Links = append(xf.Links, linkAtom{Href: f.SelfLink, Rel: "self", Type: "application/atom+xml"})
	}
	if f.Author == "" {
		xf.Author = &authorAtom{Name: "-"}
	} else {
		xf.Author = &authorAtom{Name: f.Author}
	}
	for _, e := range f.Entries {
		xf.Entries = append(xf.Entries, toAtomEntry(e))
	}
	return &xf
}

func (f *Feed) ToAtom(w io.Writer, indent string) (err error) {
	xf := toAtomFeed(f)
	if _, err = io.WriteString(w, xml.Header); err != nil {
		return
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", indent)
	return enc.Encode(&xf)
}
