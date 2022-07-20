package StreamTool

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

func findPlayerJSON(node *html.Node) bool {
	if node.Type != html.TextNode ||
		node.Parent == nil ||
		node.Parent.Data != "script" {
		return false
	}

	if strings.Contains(node.Data, `"jsUrl":"`) {
		return true
	}

	return false
}

func findManifestJSON(node *html.Node) bool {
	if node.Type != html.TextNode ||
		node.Parent == nil ||
		node.Parent.Data != "script" {
		return false
	}

	if strings.Contains(node.Data, `ytInitialPlayerResponse =`) {
		return true
	}

	return false
}

func youtubeSig(doc *html.Node, sig string) (string, error) {
	// find and parse player config
	node := findNode(doc, findPlayerJSON)
	if node == nil {
		return "", errors.New("couldn't find player json")
	}

	rxJsUrl := regexp.MustCompile(`"jsUrl":"(.+?)"`).FindAllStringSubmatch(node.Data, -1)
	if len(rxJsUrl) == 0 {
		return "", errors.New("couldn't match player js url")
	}
	jsUrl := rxJsUrl[0][1]

	// fetch player js
	resp, err := netClient.Get("https://www.youtube.com" + jsUrl)
	if err != nil {
		return "", fmt.Errorf("couldn't fetch player js: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("couldn't read player js: %w", err)
	}
	playerjs := string(body)

	// linear instruction set
	rxCipherInstructions := regexp.MustCompile(`\.split\(""\);(.*);return \w\.join\(""\)`).FindAllStringSubmatch(playerjs, 1)
	if len(rxCipherInstructions) == 0 {
		return "", errors.New("couldn't match cipher instructions")
	}
	cipherInstructions := rxCipherInstructions[0][1]

	// break down instructions into steps
	cipherSteps := regexp.MustCompile(`(\w+).(\w+)\(\w+,(\d+)\);?`).FindAllStringSubmatch(cipherInstructions, -1)
	if len(cipherSteps) == 0 {
		return "", errors.New("couldn't match cipher steps")
	}

	// find function definitions
	cipherFuncName := cipherSteps[0][1]
	rxFunctions := `(\w+):function\([\w,]+\)\{.*\},?\n?`

	rxCipherFuncBlock := regexp.MustCompile(fmt.Sprintf(`%s=\{(?:%s)+\};`, cipherFuncName, rxFunctions)).FindAllString(playerjs, 1)
	if len(rxCipherFuncBlock) == 0 {
		return "", errors.New("couldn't match cipher function block")
	}

	cipherFuncBlock := rxCipherFuncBlock[0]
	cipherFuncDefs := regexp.MustCompile(rxFunctions).FindAllStringSubmatch(cipherFuncBlock, -1)

	// determine what method each step uses
	cipherFuncs := map[string]string{}
	for _, match := range cipherFuncDefs {
		str := match[0]
		name := match[1]

		if strings.Contains(str, "splice") {
			cipherFuncs[name] = "splice"
		} else if strings.Contains(str, "reverse") {
			cipherFuncs[name] = "reverse"
		} else if strings.Contains(str, "var ") {
			cipherFuncs[name] = "swap"
		} else {
			return "", errors.New("unknown cipher transformation")
		}
	}

	if len(cipherFuncs) == 0 {
		return "", errors.New("empty cipher functions")
	}

	// run the steps and decipher
	for _, str := range cipherSteps {
		fn := str[2]
		arg := 0
		if len(str) > 3 {
			num, err := strconv.Atoi(str[3])
			if err != nil {
				return "", fmt.Errorf("couldn't convert arg number: %w", err)
			}
			arg = num
		}

		method := cipherFuncs[fn]
		if method == "splice" {
			sig = sig[arg:]
		} else if method == "reverse" {
			sig = reverseStr(sig)
		} else if method == "swap" {
			bytes := []byte(sig)
			first := bytes[0]
			bytes[0] = bytes[arg%len(bytes)]
			bytes[arg%len(bytes)] = first
			sig = string(bytes)
		}
	}

	return sig, nil
}

type youtubeFormat struct {
	Url             string
	Bitrate         int
	MimeType        string
	SignatureCipher string
}

type youtubeStreamingData struct {
	Formats         []youtubeFormat
	AdaptiveFormats []youtubeFormat
}

type youtubeVideoDetails struct {
	Title         string
	LengthSeconds string
}

type youtubeJSON struct {
	StreamingData youtubeStreamingData
	VideoDetails  youtubeVideoDetails
}

func parseYoutube(song_url string, urlRx *regexp.Regexp) ([]StreamData, error) {
	var streamData []StreamData
	streamData = append(streamData, StreamData{})
	streamData[0].URL = song_url

	// fetch url
	resp, err := netClient.Get(song_url)
	if err != nil {
		return streamData, fmt.Errorf("couldn't fetch url: %w", err)
	}
	defer resp.Body.Close()

	// parse html
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return streamData, fmt.Errorf("couldn't parse html: %w", err)
	}

	// find and parse manifest
	node := findNode(doc, findManifestJSON)
	if node == nil {
		return streamData, errors.New("couldn't find manifest json")
	}

	manifestJSON := node.Data

	// remove preceding "var ytInitialPlayerResponse = "
	firstBracket := strings.Index(manifestJSON, "{")
	manifestJSON = manifestJSON[firstBracket:]

	// remove trailing ;
	if strings.HasSuffix(manifestJSON, ";") {
		manifestJSON = manifestJSON[:len(manifestJSON)-1]
	}

	// unmarshal json
	var jsonData youtubeJSON
	err = json.Unmarshal([]byte(manifestJSON), &jsonData)
	if err != nil {
		return streamData, fmt.Errorf("couldn't parse json: %w", err)
	}

	// find most appropriate format
	formats := jsonData.StreamingData.Formats
	formats = append(formats, jsonData.StreamingData.AdaptiveFormats...)

	sort.Slice(formats, func(i int, j int) bool {
		return formats[i].Bitrate < formats[j].Bitrate
	})

	var highestAudio string
	var highestVideo string
	ciphered := false

	for _, format := range formats {
		url := format.Url
		if url == "" {
			ciphered = true
			url = format.SignatureCipher
		}

		if highestAudio == "" && strings.HasPrefix(format.MimeType, "audio/") {
			highestAudio = url
		} else if highestVideo == "" && strings.HasPrefix(format.MimeType, "video/") {
			highestVideo = url
		}
	}

	var streamURL string
	if highestAudio != "" {
		streamURL = highestAudio
	} else {
		streamURL = highestVideo
	}

	if ciphered {
		// stream url
		rxUrl := regexp.MustCompile(`url=([^&]+)`).FindAllStringSubmatch(streamURL, 1)
		if len(rxUrl) == 0 {
			return streamData, errors.New("couldn't match stream url")
		}
		raw_stream_url := rxUrl[0][1]

		stream_url, err := url.QueryUnescape(raw_stream_url)
		if err != nil {
			return streamData, fmt.Errorf("couldn't unescape stream url: %w", err)
		}

		// sig
		rxSig := regexp.MustCompile(`s=([^&]+)`).FindAllStringSubmatch(streamURL, 1)
		if len(rxSig) == 0 {
			return streamData, errors.New("couldn't match signature string")
		}
		raw_sig := rxSig[0][1]

		sig, err := url.QueryUnescape(raw_sig)
		if err != nil {
			return streamData, fmt.Errorf("couldn't unescape sig: %w", err)
		}

		// sig policy
		rxSp := regexp.MustCompile(`sp=([^&]+)`).FindAllStringSubmatch(streamURL, 1)
		if len(rxSp) == 0 {
			return streamData, errors.New("couldn't match signature policy")
		}
		sp := rxSp[0][1]

		// do the deciphering
		deciphered, err := youtubeSig(doc, sig)
		if err != nil {
			return streamData, fmt.Errorf("couldn't decipher signature: %w", err)
		}

		// reconstruct url
		escSig := url.QueryEscape(deciphered)
		streamURL = fmt.Sprintf("%s&%s=%s", stream_url, sp, escSig)
	}

	streamData[0].StreamURL = streamURL

	// video details
	videoID := urlRx.FindAllStringSubmatch(song_url, 1)[0][1]

	streamData[0].Title = jsonData.VideoDetails.Title

	num, err := strconv.ParseFloat(jsonData.VideoDetails.LengthSeconds, 64)
	if err != nil {
		return streamData, fmt.Errorf("couldn't parse duration: %w", err)
	}
	streamData[0].Duration = int(math.Ceil(num))

	streamData[0].ImageURL = fmt.Sprintf("https://i.ytimg.com/vi/%s/maxresdefault.jpg", videoID)

	return streamData, nil
}
