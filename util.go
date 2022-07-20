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
