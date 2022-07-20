package StreamTool

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

type bandcampFile struct {
	URL string `json:"mp3-128"`
}

type bandcampTrack struct {
	Title      string
	Title_Link string
	Duration   stringWrapper
	File       bandcampFile
}

type bandcampJSON struct {
	TrackInfo []bandcampTrack
}

func findBandcampJSON(node *html.Node) bool {
	if node.Data != "script" {
		return false
	}

	for _, attr := range node.Attr {
		if attr.Key == "data-tralbum" {
			return true
		}
	}

	return false
}

func parseBandcamp(url string, urlRx *regexp.Regexp) ([]StreamData, error) {
	var streamData []StreamData

	// fetch url
	resp, err := netClient.Get(url)
	if err != nil {
		return streamData, fmt.Errorf("couldn't fetch url: %w", err)
	}
	defer resp.Body.Close()

	// read pageBody as string (needed later)
	bodyReader, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return streamData, fmt.Errorf("couldn't read body: %w", err)
	}
	pageBody := string(bodyReader)

	// parse html
	doc, err := html.Parse(strings.NewReader(pageBody))
	if err != nil {
		return streamData, fmt.Errorf("couldn't parse html: %w", err)
	}

	// find script node containing json attr
	node := findNode(doc, findBandcampJSON)
	if node == nil {
		return streamData, errors.New("couldn't find json")
	}

	// extract data
	nodeData := ""
	for _, attr := range node.Attr {
		if attr.Key == "data-tralbum" {
			nodeData = attr.Val
		}
	}

	if nodeData == "" {
		return streamData, errors.New("couldn't extract node data")
	}

	// parse json
	var jsonData bandcampJSON
	err = json.Unmarshal([]byte(nodeData), &jsonData)
	if err != nil {
		return streamData, fmt.Errorf("couldn't parse json: %w", err)
	}

	// get album art
	albumArtURL := ""
	rxAlbumArt := regexp.MustCompile(`<div id="tralbumArt">\s*?<a class="popupImage" href="(.+?)">`).FindAllStringSubmatch(pageBody, 1)
	if len(rxAlbumArt) > 0 {
		// silently fail if not found, album art isn't the most important
		albumArtURL = rxAlbumArt[0][1]
	}

	// get base URL
	baseURL := urlRx.FindAllStringSubmatch(url, 1)[0][1]

	// build track list
	for _, track := range jsonData.TrackInfo {
		songData := StreamData{
			URL:       baseURL + track.Title_Link,
			Title:     track.Title,
			StreamURL: track.File.URL,
			ImageURL:  albumArtURL,
		}

		// parse duration
		num, err := strconv.ParseFloat(string(track.Duration), 64)
		if err == nil {
			// best to just silently fail here?
			songData.Duration = int(math.Ceil(num))
		}

		streamData = append(streamData, songData)
	}

	return streamData, nil
}
