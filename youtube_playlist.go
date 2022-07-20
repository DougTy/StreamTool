package StreamTool

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func findPlaylistJSON(node *html.Node) bool {
	if node.Type != html.TextNode ||
		node.Parent == nil ||
		node.Parent.Data != "script" {
		return false
	}

	if strings.Contains(node.Data, `ytInitialData =`) {
		return true
	}

	return false
}

func parseYoutubePlaylist(url string, urlRx *regexp.Regexp) ([]StreamData, error) {
	var streamData []StreamData

	// fetch url
	resp, err := netClient.Get(url)
	if err != nil {
		return streamData, fmt.Errorf("couldn't fetch url: %w", err)
	}
	defer resp.Body.Close()

	// parse html
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return streamData, errors.New("couldn't find playlist json")
	}

	// find and parse manifest
	node := findNode(doc, findPlaylistJSON)
	if node == nil {
		return streamData, errors.New("couldn't find playlist json")
	}

	playlistJSON := node.Data

	// remove preceding "var ytInitialData = "
	firstBracket := strings.Index(playlistJSON, "{")
	playlistJSON = playlistJSON[firstBracket:]

	// remove trailing ;
	if strings.HasSuffix(playlistJSON, ";") {
		playlistJSON = playlistJSON[:len(playlistJSON)-1]
	}

	// get video titles
	rawVideoTitles := regexp.MustCompile(`"title":\s*?{\s*?"runs":\s*?\[{\s*?"text":\s*?"(.+?)"\s*?}\],\s*?"accessibility"`).FindAllStringSubmatch(playlistJSON, -1)
	if len(rawVideoTitles) == 0 {
		return streamData, errors.New("couldn't match video titles")
	}

	// get video urls
	rawVideoURLs := regexp.MustCompile(`"videoIds":\s*?\["(.+?)"\]`).FindAllStringSubmatch(playlistJSON, -1)
	if len(rawVideoURLs) == 0 {
		return streamData, errors.New("couldn't match video URLs")
	}

	// transform video titles into an array
	var videoTitles []string
	for _, v := range rawVideoTitles {
		videoTitles = append(videoTitles, v[1])
	}

	// transform video URLs into an array and remove duplicates
	var lastURL string
	var videoURLs []string
	for _, v := range rawVideoURLs {
		if v[1] != lastURL {
			videoURLs = append(videoURLs, v[1])
		}
		lastURL = v[1]
	}

	// make sure both arrays are equal length
	if len(videoTitles) != len(videoURLs) {
		return streamData, errors.New("playlist data mismatch")
	}

	// format into return array
	// streamurl, data, and imageurl must be parsed later
	// this isn't done now for each song as it may take a very long time
	for i, title := range videoTitles {
		songData := StreamData{
			Title: title,
			URL:   fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoURLs[i]),
		}
		streamData = append(streamData, songData)
	}

	return streamData, nil
}
