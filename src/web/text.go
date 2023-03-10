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
	"bufio"
	"bytes"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
)

// matches href="link" or something down bad like hReF =  'link'
var tagHrefRegexp *regexp.Regexp = regexp.MustCompile(`(?i)(href)[\s]*=[\s]*("|')(.*?)("|')`)

// matches src="link" or even something along the lines of SrC    =  'link'
var tagSrcRegexp *regexp.Regexp = regexp.MustCompile(`(?i)(src)[\s]*=[\s]*("|')(.*?)("|')`)

var emailRegexp *regexp.Regexp = regexp.MustCompile(`[A-Za-z0-9._%+\-!%&?~^#$]+@[A-Za-z0-9.\-]+\.[a-zA-Z]{2,4}`)

// Fix relative link and construct an absolute one. Does nothing if the URL already looks alright
func ResolveLink(link url.URL, fromHost string) url.URL {
	var resolvedURL url.URL = link

	if !resolvedURL.IsAbs() {
		if resolvedURL.Scheme == "" {
			// add scheme
			resolvedURL.Scheme = "https"
		}

		if resolvedURL.Host == "" {
			// add host
			resolvedURL.Host = fromHost
		}
	}

	return resolvedURL
}

// Find all links on page that are specified in href attribute. Do not resolve links. Return URLs as they are on the page
func FindPageLinksDontResolve(pageBody []byte) []url.URL {
	var urls []url.URL

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

		urls = append(urls, *link)
	}

	return urls
}

// Find all links on page that are specified in href attribute
func FindPageLinks(pageBody []byte, from url.URL) []url.URL {
	urls := FindPageLinksDontResolve(pageBody)
	for index := 0; index < len(urls); index++ {
		urls[index] = ResolveLink(urls[index], from.Host)
	}

	return urls
}

// Find all links on page that are specified in "src" attribute. Do not resolve ULRs, return them as they are on page
func FindPageSrcLinksDontResolve(pageBody []byte) []url.URL {
	var urls []url.URL

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

		link, err := url.Parse(match[linkStartIndex+1 : linkEndIndex])
		if err != nil {
			continue
		}

		urls = append(urls, *link)
	}

	return urls
}

// Find all links on page that are specified in "src" attribute
func FindPageSrcLinks(pageBody []byte, from url.URL) []url.URL {
	urls := FindPageSrcLinksDontResolve(pageBody)
	for index := 0; index < len(urls); index++ {
		urls[index] = ResolveLink(urls[index], from.Host)
	}
	return urls
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

// Extract clear email addresses on the page
func FindPageEmails(pageBody []byte) []string {
	var emailAddresses []string

	var skip bool
	for _, email := range emailRegexp.FindAllString(string(pageBody), -1) {
		skip = false

		_, err := mail.ParseAddress(email)
		if err != nil {
			continue
		}

		for _, visitedEmail := range emailAddresses {
			if email == visitedEmail {
				skip = true
				break
			}
		}

		if skip {
			continue
		}

		emailAddresses = append(emailAddresses, email)
	}

	return emailAddresses
}

// Extract clear email addresses on the page and make an MX lookup
func FindPageEmailsWithCheck(pageBody []byte) []string {
	var filteredEmailAddresses []string

	emailAddresses := FindPageEmails(pageBody)
	for _, email := range emailAddresses {
		mx, err := net.LookupMX(strings.Split(email, "@")[1])
		if err != nil || len(mx) == 0 {
			continue
		}

		filteredEmailAddresses = append(filteredEmailAddresses, email)
	}

	return filteredEmailAddresses
}
