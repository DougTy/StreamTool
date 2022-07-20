package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/DougTy/StreamTool"
)

// needed as default json.Marshal will escape '&' from the url
func jsonMarshal(data interface{}) ([]byte, error) {
	buff := &bytes.Buffer{}
	enc := json.NewEncoder(buff)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")
	err := enc.Encode(data)
	return buff.Bytes(), err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("include a url to parse")
		return
	}

	url := os.Args[1]
	data, err := StreamTool.ParseURL(url)
	if err != nil {
		fmt.Println(err)
	}

	b, err := jsonMarshal(data)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(string(b))
}
