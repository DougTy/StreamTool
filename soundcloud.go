package StreamTool

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func findSoundcloudJSON(node *html.Node) bool {
	if node.Type != html.TextNode ||
		node.Parent == nil ||
		node.Parent.Data != "script" {
		return false
	}

	if strings.Contains(node.Data, "__sc_hydration =") {
		return true
	}

	return false
}

type soundcloudTranscoding struct {
	Url string
}

type soundcloudMedia struct {
	Transcodings []soundcloudTranscoding
}

type soundcloudData struct {
	Artwork_url string
	Duration    int
	Title       string
	Media       soundcloudMedia
}

type soundcloudHydratable struct {
	Hydratable string
	Data       stringWrapper
}

type soundcloudStream struct {
	Url string
}

func getSoundcloudStream(url string) (string, error) {
	// fetch json
	resp, err := netClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("couldn't fetch url: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("couldn't read body: %w", err)
	}
	response := string(body)

	// unmarshal json
	var jsonData soundcloudStream
	err = json.Unmarshal([]byte(response), &jsonData)
	if err != nil {
		return "", fmt.Errorf("couldn't parse json: %w", err)
	}

	return jsonData.Url, nil
}

func getSoundcloudClientID(doc string) (string, error) {
	rxScripts := regexp.MustCompile(`<script crossorigin src="(.+?)"></script>`)
	scripts := rxScripts.FindAllStringSubmatch(doc, -1)

	// loop in reverse as the desired script is usually last on the page
	for i := len(scripts) - 1; i >= 0; i-- {
		src := scripts[i][1]

		// fetch script
		resp, err := netClient.Get(src)
		if err != nil {
			return "", fmt.Errorf("couldn't fetch script: %w", err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("couldn't read script: %w", err)
		}
		js := string(body)

		// extract client id if found
		if strings.Contains(js, `client_id:"`) {
			rxClientID := regexp.MustCompile(`client_id:"(.+?)"`).FindAllStringSubmatch(js, 1)
			if len(rxClientID) == 0 {
				return "", errors.New("couldn't match client id")
			}

			clientID := rxClientID[0][1]
			return clientID, nil
		}
	}

	return "", errors.New("couldn't find client id")
}

func parseSoundcloud(url string, urlRx *regexp.Regexp) ([]StreamData, error) {
	var streamData []StreamData
	streamData = append(streamData, StreamData{})
	streamData[0].URL = url

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

	// find json script node
	node := findNode(doc, findSoundcloudJSON)
	if node == nil {
		return streamData, errors.New("couldn't find json")
	}
	hydrationJSON := node.Data

	// remove preceding "window.__sc_hydration = "
	firstBracket := strings.Index(hydrationJSON, "[")
	hydrationJSON = hydrationJSON[firstBracket:]

	// remove trailing ;
	if strings.HasSuffix(hydrationJSON, ";") {
		hydrationJSON = hydrationJSON[:len(hydrationJSON)-1]
	}

	// unmarshal raw data
	var jsonData []soundcloudHydratable
	err = json.Unmarshal([]byte(hydrationJSON), &jsonData)
	if err != nil {
		return streamData, fmt.Errorf("couldn't parse hydration json: %w", err)
	}

	// find and unmarshal sound data table
	var soundData soundcloudData
	found := false
	for _, table := range jsonData {
		if table.Hydratable == "sound" {
			err = json.Unmarshal([]byte(table.Data), &soundData)
			if err != nil {
				return streamData, fmt.Errorf("couldn't parse sound data json: %w", err)
			}

			found = true
			break
		}
	}

	if !found {
		return streamData, errors.New("couldn't find sound hydration")
	}

	// get first progressive transcoding
	format_url := ""
	for _, format := range soundData.Media.Transcodings {
		if strings.Contains(format.Url, "/progressive") {
			format_url = format.Url
			break
		}
	}

	if format_url == "" {
		return streamData, errors.New("couldn't find progressive stream")
	}

	// get client id
	clientID, err := getSoundcloudClientID(pageBody)
	if err != nil {
		return streamData, fmt.Errorf("couldn't get client id: %w", err)
	}

	// get stream url
	fetch_url := format_url + "?client_id=" + clientID

	stream_url, err := getSoundcloudStream(fetch_url)
	if err != nil {
		return streamData, fmt.Errorf("couldn't get stream url: %w", err)
	}

	streamData[0].StreamURL = stream_url

	// media info
	streamData[0].Title = soundData.Title
	streamData[0].Duration = int(math.Ceil(float64(soundData.Duration) / 1000.0))
	streamData[0].ImageURL = soundData.Artwork_url

	return streamData, nil
}
