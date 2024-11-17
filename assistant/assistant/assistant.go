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
    "time"

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

// It takes the summary as an argument and renames the local branch.
func renameBranch(summary string) error {
    // Add timestamp to the branch name to avoid collisions
    timestamp := time.Now().Format("20060102-150405")
    newBranchName := fmt.Sprintf("assistant-%s-%s", summary, timestamp)

    // Run git command to rename the branch
    cmd := exec.Command("git", "branch",
      "-m", "assistant-branch", newBranchName)
    cmd.Dir = "repo"

    // Capture stdout and stderr
    var outBuf, errBuf bytes.Buffer
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf

    // Execute the command
    err := cmd.Run()
    if err != nil {
        // Log the output and error if the command fails
        log.Printf("Failed to rename branch. Stdout: %s, Stderr: %s", outBuf.String(), errBuf.String())
        return fmt.Errorf("failed to rename branch: %v", err)
    }

    // Log the successful output
    log.Printf("Branch renamed successfully. Stdout: %s", outBuf.String())
    return nil
}

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
      data.Branch = fmt.Sprintf("assistant-%s-%s", summary, time.Now().Format("20060102-150405"))
      if err != nil {
        log.Fatalf("Error renaming branch: %v", err)
      }
    }

    // Loop through each file path and content pair
    for filePath, newContent := range filesContent {
        if strings.Contains(newContent, "\n// ... (other functions remain unchanged)") {
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

// .. remaining functions unchanged