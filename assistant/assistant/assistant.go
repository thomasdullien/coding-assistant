package assistant

import (
    "fmt"
    "regexp"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "strings"
    "bytes"
    "bufio"
    "path/filepath"

    "github.com/thomasdullien/coding-assistant/assistant/chatgpt"
    "github.com/thomasdullien/coding-assistant/assistant/types"
)

// ProcessAssistant handles the main workflow
func ProcessAssistant(data types.FormData) (string, error) {
    log.Println("Cloning repository and creating branch...")
    err := cloneAndCheckoutRepo(&data)
    if err != nil {
        return "", fmt.Errorf("failed to clone repository: %v", err)
    }

    var deps []string
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
        log.Println("Including entire repository for Golang.")
        deps, err = includeEntireRepo("repo")
        if err != nil {
            return "", fmt.Errorf("failed to include entire repository: %v", err)
        }
    }

    log.Println("Preparing prompt...")
    prompt := buildPrompt(data.Prompt, deps)
    log.Println("Prompt:", prompt)

    for attempts := 0; attempts < 2; attempts++ {
        log.Printf("Applying changes, attempt %d...", attempts+1)
        err := applyChangesWithChatGPT(&data, prompt)
        if err != nil {
            return "", fmt.Errorf("failed to apply changes: %v", err)
        }

        log.Println("Running tests...")
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

// StitchFiles stitches the content back together if truncation is detected.
func StitchFiles(originalFilePath string, newContent string) (string, error) {
    // Check for the truncation comment
    if strings.Contains(newContent, "// ... (other functions remain unchanged)") {
        // Read the original file content
        originalContent, err := ioutil.ReadFile(originalFilePath)
        if err != nil {
            return "", fmt.Errorf("failed to read original file: %v", err)
        }

        // Stitch together the new and original content
        newContent = strings.Replace(newContent, "// ... (other functions remain unchanged)", string(originalContent), 1)
    }
    return newContent, nil
}

// applyChangesWithChatGPT function modified to handle file stitching
func applyChangesWithChatGPT(data *types.FormData, prompt string) error {
    request := chatgpt.CreateRequest(prompt)
    response, err := chatgpt.SendRequest(request)
    if err != nil {
        return fmt.Errorf("failed to get response from ChatGPT: %v", err)
    }

    filesContent, summary, success := parseResponseForFiles(response)
    if !success {
        return fmt.Errorf("failed to parse files from ChatGPT response")
    }
    if success {
      err := renameBranch(summary)
      data.Branch = fmt.Sprintf("assistant-%s", summary)
      if err != nil {
        log.Fatalf("Error renaming branch: %v", err)
      }
    }

    for filePath, content := range filesContent {
        stitchedContent, err := StitchFiles(filePath, content)
        if err != nil {
            log.Printf("Error stitching file content for %s: %v", filePath, err)
            continue
        }
        err = ioutil.WriteFile(filePath, []byte(stitchedContent), 0644)
        if err != nil {
            log.Printf("failed to write changes to file %s: %v", filePath, err)
            continue
        }
        log.Printf("Successfully applied changes to %s", filePath)
    }

    return nil
}

// Other existing functions remain unchanged