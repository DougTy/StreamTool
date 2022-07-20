package StreamTool

import (
	"net/http"
	"time"

	"golang.org/x/net/html"
)

// find html node with callback boolean
func findNode(node *html.Node, cb func(*html.Node) bool) *html.Node {
	if node == nil || cb(node) {
		return node
	}

	if n := findNode(node.FirstChild, cb); n != nil {
		return n
	}

	return findNode(node.NextSibling, cb)
}

// wrapper for unmarshalling variable types into a string
type stringWrapper string

func (w *stringWrapper) UnmarshalJSON(data []byte) (err error) {
	*w = stringWrapper(data)
	return nil
}

// reverse string
func reverseStr(str string) string {
	var res string
	for _, char := range str {
		res = string(char) + res
	}
	return res
}

// custom http client with timeout
var netClient = &http.Client{
	Timeout: time.Second * 10,
}
