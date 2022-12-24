/*
	websurf - surf the web for data recursively
	Copyright (C) 2022 Kasyanov Nikolay Alexeyevich (Unbewohnte)

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package web

import (
	"bytes"
	"net/url"
	"strings"
	"unbewohnte/websurf/logger"

	"golang.org/x/net/html"
)

func hasImageExtention(url string) bool {
	var extentions []string = []string{
		".jpeg",
		".jpg",
		".jpe",
		".png",
		".ppm",
		".svg",
	}

	for _, extention := range extentions {
		if strings.HasSuffix(url, extention) {
			return true
		}
	}

	return false
}

// Tries to find images' URLs on the page
func FindPageImages(pageBody []byte, hostname string) []string {
	var urls []string

	tokenizer := html.NewTokenizer(bytes.NewReader(pageBody))
	for {
		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			return urls

		case html.StartTagToken:
			token := tokenizer.Token()

			if token.Data != "img" && token.Data != "a" {
				continue
			}

			// recheck
			for _, attribute := range token.Attr {
				if attribute.Key != "src" && attribute.Key != "href" {
					continue
				}

				var imageURLString string

				imageURL, err := url.Parse(attribute.Val)
				if err != nil {
					logger.Error("Failed to parse URL %s: %s", attribute.Val, err)
					continue
				}
				imageURLString = imageURL.String()

				if strings.HasPrefix(imageURLString, "//") {
					imageURLString = "http:" + imageURLString
				}

				if hasImageExtention(imageURLString) {
					urls = append(urls, imageURLString)
				}
			}
		}
	}
}
