/*
	Wecr - crawl the web for data
	Copyright (C) 2022, 2023 Kasyanov Nikolay Alexeyevich (Unbewohnte)

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
	"net/url"
	"strings"
)

func HasImageExtention(url string) bool {
	for _, extention := range ImageExtentions {
		if strings.HasSuffix(url, extention) {
			return true
		}
	}

	return false
}

// Tries to find images' URLs on the page
func FindPageImages(pageBody []byte, from *url.URL) []string {
	var urls []string

	// for every element that has "src" attribute
	for _, match := range tagSrcRegexp.FindAllString(string(pageBody), -1) {
		var linkStartIndex int
		var linkEndIndex int

		linkStartIndex = strings.Index(match, "\"")
		if linkStartIndex == -1 {
			linkStartIndex = strings.Index(match, "'")
			if linkStartIndex == -1 {
				continue
			}

			linkEndIndex = strings.LastIndex(match, "'")
			if linkEndIndex == -1 {
				continue
			}
		} else {
			linkEndIndex = strings.LastIndex(match, "\"")
			if linkEndIndex == -1 {
				continue
			}
		}

		if linkEndIndex <= linkStartIndex+1 {
			continue
		}

		link, err := url.Parse(match)
		if err != nil {
			continue
		}

		linkResolved := ResolveLink(link, from.Host)
		if HasImageExtention(linkResolved) {
			urls = append(urls, linkResolved)
		}
	}

	// for every "a" element as well
	for _, match := range tagHrefRegexp.FindAllString(string(pageBody), -1) {
		var linkStartIndex int
		var linkEndIndex int

		linkStartIndex = strings.Index(match, "\"")
		if linkStartIndex == -1 {
			linkStartIndex = strings.Index(match, "'")
			if linkStartIndex == -1 {
				continue
			}

			linkEndIndex = strings.LastIndex(match, "'")
			if linkEndIndex == -1 {
				continue
			}
		} else {
			linkEndIndex = strings.LastIndex(match, "\"")
			if linkEndIndex == -1 {
				continue
			}
		}

		if linkEndIndex <= linkStartIndex+1 {
			continue
		}

		link, err := url.Parse(match[linkStartIndex+1 : linkEndIndex])
		if err != nil {
			continue
		}

		linkResolved := ResolveLink(link, from.Host)
		if HasImageExtention(linkResolved) {
			urls = append(urls, linkResolved)
		}
	}

	// return discovered mutual image urls from <img> and <a> tags
	return urls
}
