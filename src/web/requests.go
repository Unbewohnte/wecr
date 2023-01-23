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
	"io"
	"net/http"
	"os"
	"time"
)

// Get page data coming from url with optional user agent and timeout
func GetPage(url string, userAgent string, timeOutMs uint64) ([]byte, error) {
	http.DefaultClient.CloseIdleConnections()
	http.DefaultClient.Timeout = time.Duration(timeOutMs * uint64(time.Millisecond))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	pageData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return pageData, nil
}

// Fetch file from url and save to file at filePath
func FetchFile(url string, userAgent string, timeOutMs uint64, filePath string) error {
	client := http.Client{}
	client.Timeout = time.Duration(timeOutMs)
	client.CheckRedirect = http.DefaultClient.CheckRedirect
	client.Transport = http.DefaultClient.Transport

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Close = true

	response, err := client.Do(req)
	if err != nil {
		return nil
	}
	response.Close = true
	defer response.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, _ = io.Copy(file, response.Body)

	client.CloseIdleConnections()

	return nil
}
