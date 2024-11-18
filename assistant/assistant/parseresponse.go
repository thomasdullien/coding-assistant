package assistant

import (
  "regexp"
  "strings"
)

// parseResponseForFiles extracts the content for each file and a summary string from the response.
// It returns a map of file paths and their contents, the extracted summary string, and a boolean indicating success.
func parseResponseForFiles(response string) (map[string]string, string, bool) {
    filesContent := make(map[string]string)

    // Regex to match the START and END delimiters with file paths, ensuring they 
    //are surrounded by newlines
    startRegex := regexp.MustCompile(`(?m)^\s*/\* START OF FILE: (.*?) \*/\s*$`)
    endRegex := regexp.MustCompile(`(?m)^\s*/\* END OF FILE: .*? \*/\s*$`)
    
    // Regex to match "Summary: $summary", where $summary contains only alphanumeric characters and dashes
    summaryRegex := regexp.MustCompile(`Summary: ([a-zA-Z0-9-]+)`)
    summaryMatch := summaryRegex.FindStringSubmatch(response)
    var summary string
    if len(summaryMatch) > 1 {
        summary = summaryMatch[1]
    } else {
        return nil, "", false // No summary found
    }

    // Find all start matches and iterate over them
    startMatches := startRegex.FindAllStringSubmatchIndex(response, -1)
    if len(startMatches) == 0 {
        return nil, "", false // No files found
    }

    for _, startMatch := range startMatches {
        end := startMatch[1]
        filename := response[startMatch[2]:startMatch[3]] // Extract filename from capture group

        // Find the corresponding end delimiter starting from the end of the start delimiter
        endMatch := endRegex.FindStringIndex(response[end:])
        if endMatch == nil {
            continue // If there's no matching END delimiter, skip this file
        }

        // Calculate actual end position in the original string
        contentStart := end
        contentEnd := end + endMatch[0]
        content := strings.TrimSpace(response[contentStart:contentEnd])

        // Store the filename and its content in the map
        filesContent[filename] = content
    }

    return filesContent, summary, true
}


