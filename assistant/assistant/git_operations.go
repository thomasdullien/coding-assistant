package assistant

import (
    "bytes"
    "fmt"
    "log"
    "os"
    "os/exec"
    "time"

    "github.com/thomasdullien/coding-assistant/assistant/types"
)

func cloneAndCheckoutRepo(data *types.FormData) error {
    // Remove the existing "repo" directory if it exists
    if _, err := os.Stat("repo"); err == nil {
        err = os.RemoveAll("repo")
        if err != nil {
            return fmt.Errorf("failed to remove existing repo directory: %v", err)
        }
    }

    // Prepare the git clone command
    cmd := exec.Command("git", "clone", data.RepoURL, "repo")

    // Capture stdout and stderr
    var outBuf, errBuf bytes.Buffer
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf

    // Run the command
    err := cmd.Run()
    if err != nil {
        return fmt.Errorf("git clone failed: %v\nstdout: %s\nstderr: %s", err, outBuf.String(), errBuf.String())
    }

    // Checkout the new branch
    cmd = exec.Command("git", "checkout", "-b", data.Branch)
    cmd.Dir = "repo"
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf
    err = cmd.Run()
    if err != nil {
        return fmt.Errorf("git checkout failed: %v\nstdout: %s\nstderr: %s", err, outBuf.String(), errBuf.String())
    }

    return nil
}

// commitAndPush stages changes, commits them, and pushes to the remote repository.
func commitAndPush(data *types.FormData) error {
    // Run `git add .` to stage all changes
    addCmd := exec.Command("git", "add", ".")
    var addOutBuf, addErrBuf bytes.Buffer
    addCmd.Stdout = &addOutBuf
    addCmd.Stderr = &addErrBuf
    addCmd.Dir = "repo"

    if err := addCmd.Run(); err != nil {
        log.Printf("Failed to add changes. Stdout: %s, Stderr: %s", addOutBuf.String(), addErrBuf.String())
        return fmt.Errorf("failed to add changes: %v", err)
    }

    // Run `git commit -m "Applying user prompt changes"` to create a commit
    commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Applying changes from user prompt: %s", data.Prompt))
    var commitOutBuf, commitErrBuf bytes.Buffer
    commitCmd.Stdout = &commitOutBuf
    commitCmd.Stderr = &commitErrBuf
    commitCmd.Dir = "repo"

    if err := commitCmd.Run(); err != nil {
        log.Printf("Failed to commit changes. Stdout: %s, Stderr: %s", commitOutBuf.String(), commitErrBuf.String())
        return fmt.Errorf("failed to commit changes: %v", err)
    }

    // Run `git push -u origin <branch>` to push the changes to the remote branch
    log.Println("data.Branch is", data.Branch)
    pushCmd := exec.Command("git", "push", "-u", "origin", data.Branch)
    var pushOutBuf, pushErrBuf bytes.Buffer
    pushCmd.Stdout = &pushOutBuf
    pushCmd.Stderr = &pushErrBuf
    pushCmd.Dir = "repo"

    if err := pushCmd.Run(); err != nil {
        log.Printf("Failed to push changes. Stdout: %s, Stderr: %s", pushOutBuf.String(), pushErrBuf.String())
        return fmt.Errorf("failed to push changes: %v", err)
    }

    log.Println("Changes committed and pushed successfully.")
    return nil
}

// createPullRequest creates a pull request using the GitHub CLI (`gh`) command.
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