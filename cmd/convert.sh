#!/bin/bash

# usage:
#	./convert.sh "url-to-stream"
# requires jq and ffmpeg

OUT=~/Downloads

JSON=$(go run . $1)
TITLE=$(echo $JSON | jq --raw-output '.[0].Title' | sed 's/\//_/g')
URL=$(echo $JSON | jq --raw-output '.[0].StreamURL')

ffmpeg -i "$URL" -vn -acodec libmp3lame -ar 48000 -ac 2 -b:a 320k "$OUT/$TITLE.mp3"
