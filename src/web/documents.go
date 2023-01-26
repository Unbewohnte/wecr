package web

import (
	"net/url"
	"strings"
)

func HasDocumentExtention(url string) bool {
	for _, extention := range DocumentExtentions {
		if strings.HasSuffix(url, extention) {
			return true
		}
	}

	return false
}

// Tries to find docs' URLs on the page
func FindPageDocuments(pageBody []byte, from *url.URL) []string {
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

		link, err := url.Parse(match[linkStartIndex+1 : linkEndIndex])
		if err != nil {
			continue
		}

		linkResolved := ResolveLink(link, from.Host)
		if HasDocumentExtention(linkResolved) {
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
		if HasDocumentExtention(linkResolved) {
			urls = append(urls, linkResolved)
		}
	}

	// return discovered doc urls
	return urls
}
