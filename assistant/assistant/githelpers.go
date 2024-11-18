package assistant

import (
  "time"
  "fmt"
  "os/exec"
  "log"
  "bytes"
  "os"

  "github.com/thomasdullien/coding-assistant/assistant/types"
)



// It takes the summary as an argument and renames the local branch.
func renameBranch(summary string) error {
    // Append timestamp to the branch name to avoid collision
    timestamp := time.Now().Format("20060102150405") // Format: YYYYMMDDHHMMSS
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
// Logs detailed output in case of errors for each command.
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


