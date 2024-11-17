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
            prlink, err := createPullRequest(&data, data.Prompt) // Pass the prompt to include in PR body
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

// ... rest of the functions remain unchanged ...

// createPullRequest creates a pull request using the GitHub CLI (`gh`) command.
// Logs detailed output in case of errors.
func createPullRequest(data *types.FormData, userPrompt string) (string, error) {
    // Prepare the `gh` command to create a pull request
    body := fmt.Sprintf("Please review the automated changes.\n\nUser Prompt: %s", userPrompt)
    cmd := exec.Command("gh", "pr", "create", "--title", "Automated Changes", "--body", body)
    cmd.Dir = "repo" // Set the working directory to the local repo

    // Capture stdout and stderr
    var outBuf, errBuf bytes.Buffer
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf

    // Execute the command
    err := cmd.Run()
    if err != nil {
        // Log the output and error if the command fails
        log.Printf("Failed to create pull request. Stdout: %s, Stderr: %s", outBuf.String(), errBuf.String())
        return "", fmt.Errorf("failed to create pull request: %v", err)
    }

    // Log the successful output
    log.Printf("Pull request created successfully. Stdout: %s", outBuf.String())
    return outBuf.String(), nil
}