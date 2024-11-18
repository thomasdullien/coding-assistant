package assistant

import (
  "fmt"
  "os/exec"
  "bytes"
  "log"
  "github.com/thomasdullien/coding-assistant/assistant/types"
)

// createPullRequest creates a pull request using the GitHub CLI (`gh`) command.
// Logs detailed output in case of errors.
func createPullRequest(data *types.FormData) (string, error) {
    // Prepare the `gh` command to create a pull request
    cmd := exec.Command("gh", "pr", "create", "--title", fmt.Sprintf("Automated Changes based on: %s", data.Prompt), "--body", fmt.Sprintf("Automated changes based on: %s", data.Prompt))
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


