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

// It takes the summary as an argument and renames the local branch.
func renameBranch(summary string) error {
    newBranchName := fmt.Sprintf("assistant-%s", summary)

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
      data.Branch = fmt.Sprintf("assistant-%s", summary)
      if err != nil {
        log.Fatalf("Error renaming branch: %v", err)
      }
    }

    // Loop through each file path and content pair
    for filePath, newContent := range filesContent {
        if strings.Contains(newContent, "// ... (other functions remain unchanged)") {
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

    // Extract sections before and after the placeholder
    beforePlaceholder := parts[0]
    afterPlaceholder := parts[1]

    // Find the matching position in the original file for the beforePlaceholder
    beforeIndex := strings.Index(originalContent, beforePlaceholder)
    if beforeIndex == -1 {
        return "", fmt.Errorf("could not find matching section for 'before' in original file %s", filePath)
    }

    // Find the remaining content after the placeholder in the original file
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

    // Regex to match the START and END delimiters with file paths
    startRegex := regexp.MustCompile(`/\* START OF FILE: (.*?) \*/`)
    endRegex := regexp.MustCompile(`/\* END OF FILE: .*? \*/`)

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

// calculateDependencies runs `gcc -M` on the input files and parses the output to extract dependencies.
func calculateDependencies(files []string) ([]string, error) {
    // Prepend "repo/" to each file in the files slice
    for i, file := range files {
        files[i] = "repo/" + file
    }

    // Prepare the gcc command with the -M flag and the input files
    args := append([]string{"-M"}, files...)
    cmd := exec.Command("gcc", args...)

    // Capture stdout and stderr
    var outBuf, errBuf bytes.Buffer
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf

    // Run the command
    err := cmd.Run()
    if err != nil {
        return nil, fmt.Errorf("gcc -M failed: %v\nstderr: %s", err, errBuf.String())
    }

    // Parse the output, filtering out lines that end with ':' or don't contain "repo"
    scanner := bufio.NewScanner(&outBuf)
    var dependencies []string

    for scanner.Scan() {
        line := scanner.Text()
        // Split the line by spaces to handle the output format
        parts := strings.Fields(line)

        for _, part := range parts {
            // Remove any trailing commas or backslashes that `gcc -M` might include
            part = strings.TrimSuffix(part, ",")
            part = strings.TrimSuffix(part, "\\")

            // Filter out any strings that end with ':' or don't contain "repo"
            if !strings.HasSuffix(part, ":") && strings.Contains(part, "repo") {
                dependencies = append(dependencies, part)
            }
        }
    }

    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("error reading gcc -M output: %v", err)
    }

    return dependencies, nil
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

    // Run `git commit -m "Applying requested changes"` to create a commit
    commitCmd := exec.Command("git", "commit", "-m", "Applying requested changes")
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
// Logs detailed output in case of errors.
func createPullRequest(data *types.FormData) (string, error) {
    // Prepare the `gh` command to create a pull request
    cmd := exec.Command("gh", "pr", "create", "--title", "Automated Changes", "--body", "Please review the automated changes.")
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

func runTests() bool {
    cmd := exec.Command("make", "tests")
    cmd.Dir = "repo"
    err := cmd.Run()
    return err == nil
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



