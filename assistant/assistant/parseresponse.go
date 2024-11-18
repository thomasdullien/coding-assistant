package assistant

import (
    "fmt"
    "io/ioutil"
    "strings"
)

// spliceFileWithOriginal reads the original file, merges it with new content, and returns the spliced content.
func spliceFileWithOriginal(filePath, newContent string) (string, error) {
    // Read the original file from the repository
    originalContentBytes, err := ioutil.ReadFile(filePath)
    if err != nil {
        return "", fmt.Errorf("failed to read original file %s: %v", filePath, err)
    }
    originalContent := string(originalContentBytes)

    // Split the response into sections before and after the placeholder
    parts := strings.Split(newContent, "// ... (other functions remain unchanged)")
    if len(parts) != 2 {
        return "", fmt.Errorf("unexpected content format, placeholder not properly split in %s", filePath)
    }

    beforePlaceholder := parts[0]
    afterPlaceholder := parts[1]

    // Extract the last few lines of "beforePlaceholder"
    beforeLines := strings.Split(beforePlaceholder, "\n")
    linesToMatch := 5
    if len(beforeLines) < linesToMatch {
        linesToMatch = len(beforeLines)
    }
    lastFewLines := strings.Join(beforeLines[len(beforeLines)-linesToMatch:], "\n")

    // Search for the last few lines in the original file
    beforeIndex := strings.LastIndex(originalContent, lastFewLines)
    if beforeIndex == -1 {
        return "", fmt.Errorf("could not find matching section for 'before' in original file %s", filePath)
    }

    // Extract the content from the original file after the placeholder
    afterIndex := strings.Index(originalContent[beforeIndex:], afterPlaceholder)
    if afterIndex == -1 {
        return "", fmt.Errorf("could not find matching section for 'after' in original file %s", filePath)
    }

    // Splice the sections together
    splicedContent := originalContent[:beforeIndex] + beforePlaceholder + originalContent[beforeIndex+afterIndex:] + afterPlaceholder

    return splicedContent, nil
}