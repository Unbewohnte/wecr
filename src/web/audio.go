/*
	Wecr - crawl the web for data
	Copyright (C) 2023 Kasyanov Nikolay Alexeyevich (Unbewohnte)

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
)

// Tries to find audio URLs on the page
func FindPageAudio(pageBody []byte, from url.URL) []url.URL {
	var urls []url.URL

	// for every element that has "src" attribute
	for _, link := range FindPageSrcLinks(pageBody, from) {
		if HasAudioExtention(link.EscapedPath()) {
			urls = append(urls, link)
		}
	}

	// for every "a" element as well
	for _, link := range FindPageLinks(pageBody, from) {
		if HasAudioExtention(link.EscapedPath()) {
			urls = append(urls, link)
		}
	}

	return urls
}
