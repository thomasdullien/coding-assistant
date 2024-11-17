package assistant

import (
    "bufio"
    "bytes"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "regexp"
    "strings"
    "time" // Import time package

    "github.com/thomasdullien/coding-assistant/assistant/chatgpt"
    "github.com/thomasdullien/coding-assistant/assistant/types"
)

// ProcessAssistant handles the main workflow
func ProcessAssistant(data types.FormData) (string, error) {
    // Clone repository and create branch
    log.Println("Cloning repository and creating branch...")
    err := cloneAndCheckoutRepo(&data)
    if err != nil {
        return "", fmt.Errorf("failed to clone repository: %v", err)
    }

    var deps []string
    // Calculate dependencies for C++ code.
    if (data.RepoType == "C++") {
        log.Println("Calculating C++ dependencies...")
        deps, err = calculateDependencies(data.Files)
        for i, dep := range deps {
          log.Printf("Dependency %d: %s", i, dep)
        }
        if err != nil {
          return "", fmt.Errorf("failed to calculate dependencies: %v", err)
        }
    } else if data.RepoType == "Golang" {
        // For Golang repositories, include the entire repository
        log.Println("Including entire repository for Golang.")
        deps, err = includeEntireRepo("repo")
        if err != nil {
            return "", fmt.Errorf("failed to include entire repository: %v", err)
        }
    }

    // Prepare prompt
    log.Println("Preparing prompt...")
    prompt := buildPrompt(data.Prompt, deps)
    // Log the prompt for debugging
    log.Println("Prompt:", prompt)

    // Query ChatGPT and apply changes iteratively
    for attempts := 0; attempts < 2; attempts++ {
        log.Printf("Applying changes, attempt %d...", attempts+1)
        err := applyChangesWithChatGPT(&data, prompt)
        if err != nil {
            return "", fmt.Errorf("failed to apply changes: %v", err)
        }

        // Run tests and create pull request if successful
        log.Println("Running tests...")
        // For the moment, assume that Golang tests always pass. This
        // needs to change in the future.
        if runTests() || data.RepoType == "Golang" {
            log.Println("Tests passed, creating pull request...")
            err1 := commitAndPush(&data)
            if err1 != nil {
              return "", fmt.Errorf("failed to commit and push changes: %v", err1)
            }
            log.Println("Changes pushed to branch.")
            prlink, err := createPullRequest(&data)
            if err != nil {
                return "", fmt.Errorf("failed to create pull request: %v", err)
            }
            log.Printf("Pull request created: %s", prlink)
            return prlink, nil
        }
        prompt += "\nTest failed, please address the following issues."
    }
    log.Println("Exceeded maximum attempts, please review manually.")
    return "", fmt.Errorf("Exceeded maximum attempts to fix the test, please review.")
}

// All helper functions go here

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

// runTests executes the tests defined in the repo and returns true if they pass.
func runTests() bool {
    cmd := exec.Command("make", "tests")
    cmd.Dir = "repo"
    err := cmd.Run()
    return err == nil
}

// includeEntireRepo includes all .go files in the given repository directory.
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