package StreamTool

import (
	"errors"
	"regexp"
)

type StreamData struct {
	URL       string
	Title     string
	StreamURL string
	Duration  int
	ImageURL  string
}

var linkParsers = map[*regexp.Regexp](func(string, *regexp.Regexp) ([]StreamData, error)){
	regexp.MustCompile(`https:\/\/(?:www\.|m\.)?youtube\.com\/watch\?v=(.+)`): parseYoutube,
	regexp.MustCompile(`https:\/\/youtu\.be\/(.+)`):                           parseYoutube,
}

func ParseURL(url string) ([]StreamData, error) {
	for rx, parser := range linkParsers {
		if rx.MatchString(url) {
			data, err := parser(url, rx)
			return data, err
		}
	}

	return []StreamData{}, errors.New("not an accepted url")
}
