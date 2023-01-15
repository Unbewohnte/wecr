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

package utilities

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"unbewohnte/wecr/web"
)

// Extracts data from the output JSON file and puts it in a new file with separators between each entry
func ExtractDataFromOutput(inputFilename string, outputFilename string, separator string, keepDuplicates bool) error {
	inputFile, err := os.Open(inputFilename)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputFilename)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	var processedData []string

	decoder := json.NewDecoder(inputFile)
	for {
		var result web.Result

		err := decoder.Decode(&result)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		for _, dataEntry := range result.Data {
			var skip = false
			if !keepDuplicates {
				for _, processedEntry := range processedData {
					if dataEntry == processedEntry {
						skip = true
						break
					}
				}

				if skip {
					continue
				}
				processedData = append(processedData, dataEntry)
			}

			outputFile.WriteString(fmt.Sprintf("%s%s", dataEntry, separator))
		}
	}

	return nil
}
