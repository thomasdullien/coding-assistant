package assistant

import (
    "bytes"
    "bufio"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strings"
    "time"

    "github.com/thomasdullien/coding-assistant/assistant/chatgpt"
    "github.com/thomasdullien/coding-assistant/assistant/types"
)

// applyChangesWithChatGPT sends a prompt to ChatGPT, retrieves the response, and applies any changes
// specified in the response to the relevant files in the local repository.
func applyChangesWithChatGPT(data *types.FormData, prompt string) error {
    // Create a ChatGPT request with the initial prompt
    request := chatgpt.CreateRequest(prompt)

    // Send the request to ChatGPT and get a response
    response, err := chatgpt.SendRequest(request)
    if err != nil {
        return fmt.Errorf("failed to get response from ChatGPT: %v", err)
    }

    // Parse the response to extract file contents based on delimiters
    filesContent, summary, success := parseResponseForFiles(response)
    if !success {
        return fmt.Errorf("failed to parse files from ChatGPT response")
    }
    
    if success {
        err := renameBranch(summary)
        data.Branch = fmt.Sprintf("assistant-%s-%s", summary, time.Now().Format("20060102150405")) // Update branch name with timestamp
        if err != nil {
            log.Fatalf("Error renaming branch: %v", err)
        }
    }

    // Loop through each file path and content pair
    for filePath, newContent := range filesContent {
        if strings.Contains(newContent, "\n// ... remaining functions unchanged") {
            // Handle splicing
            log.Printf("Detected placeholder in %s, splicing content...", filePath)
            updatedContent, spliceErr := spliceFileWithOriginal(filePath, newContent)
            if spliceErr != nil {
                return fmt.Errorf("failed to splice file %s: %v", filePath, spliceErr)
            }
            newContent = updatedContent
        }

        // Write the updated content to the file
        err := ioutil.WriteFile(filePath, []byte(newContent), 0644)
        if err != nil {
            log.Printf("failed to write changes to file %s: %v", filePath, err)
            continue
        }
        log.Printf("Successfully applied changes to %s", filePath)
    }
    return nil
}

// Other related functions like renameBranch, spliceFileWithOriginal, etc. go here.
// ...

// parseResponseForFiles extracts the content for each file and a summary string from the response.
// It returns a map of file paths and their contents, the extracted summary string, and a boolean indicating success.
func parseResponseForFiles(response string) (map[string]string, string, bool) {
    filesContent := make(map[string]string)

    // Regex to match the START and END delimiters with file paths, ensuring they 
    // are surrounded by newlines
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

func includeEntireRepo(repoPath string) ([]string, error) {
    var files []string
    err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() && strings.HasSuffix(path, ".go") {
            files = append(files, path)
        }
        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("failed to walk repository: %v", err)
    }
    return files, nil
}

// buildPrompt generates a prompt that includes the user's request and the contents of each dependency file.
func buildPrompt(userPrompt string, deps []string) string {
    var builder strings.Builder

    // Start with the user prompt
    builder.WriteString(userPrompt)
    builder.WriteString("\n\nDependencies:\n")

    // Loop through each dependency file
    for _, dep := range deps {
        // Add start delimiter
        builder.WriteString(fmt.Sprintf("\n/* START OF FILE: %s */\n", dep))

        // Read the content of the dependency file
        content, err := ioutil.ReadFile(dep)
        if err != nil {
            builder.WriteString(fmt.Sprintf("Error reading file: %s\n", err))
        } else {
            builder.Write(content)
        }

        // Add end delimiter
        builder.WriteString(fmt.Sprintf("\n/* END OF FILE: %s */\n\n", dep))
    }

    return builder.String()
}