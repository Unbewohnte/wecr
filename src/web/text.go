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
	"bufio"
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func FindPageLinks(pageBody []byte) []string {
	var urls []string

	tokenizer := html.NewTokenizer(bytes.NewReader(pageBody))
	for {
		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			return urls

		case html.StartTagToken:
			token := tokenizer.Token()

			if token.Data != "a" {
				continue
			}

			// recheck
			for _, attribute := range token.Attr {
				if attribute.Key != "href" {
					continue
				}

				if strings.HasPrefix(attribute.Val, "http") {
					urls = append(urls, attribute.Val)
				}
			}
		}
	}
}

// Tries to find a certain string in page. Returns true if such string has been found
func IsTextOnPage(text string, ignoreCase bool, pageBody []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(pageBody))

	for scanner.Scan() {
		lineBytes := scanner.Bytes()

		if !ignoreCase {
			if bytes.Contains(lineBytes, []byte(text)) {
				return true
			}
		} else {
			if strings.Contains(strings.ToLower(string(lineBytes)), strings.ToLower(text)) {
				return true
			}
		}
	}

	return false
}

// Tries to find a string matching given regexp in page. Returns an array of found
func FindPageRegexp(re *regexp.Regexp, pageBody []byte) []string {
	return re.FindAllString(string(pageBody), -1)
}