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

    "assistant/chatgpt"
    "assistant/types"
)

// ProcessAssistant handles the main workflow
func ProcessAssistant(data types.FormData) {
    // Clone repository and create branch
    log.Println("Cloning repository and creating branch...")
    err := cloneAndCheckoutRepo(data)
    if err != nil {
        log.Fatalf("Failed to clone repository: %v", err)
    }

    // Calculate dependencies
    log.Println("Calculating dependencies...")
    deps, err := calculateDependencies(data.Files)
    for i, dep := range deps {
      log.Printf("Dependency %d: %s", i, dep)
    }
    if err != nil {
        log.Fatalf("Failed to calculate dependencies: %v", err)
    }

    // Prepare prompt
    log.Println("Preparing prompt...")
    prompt := buildPrompt(data.Prompt, deps)
    // Log the prompt for debugging
    log.Println("Prompt:", prompt)

    // Query ChatGPT and apply changes iteratively
    for attempts := 0; attempts < 5; attempts++ {
        log.Printf("Applying changes, attempt %d...", attempts+1)
        err := applyChangesWithChatGPT(data, prompt)
        if err != nil {
            log.Fatalf("Failed to apply changes: %v", err)
        }

        // Run tests and create pull request if successful
        log.Println("Running tests...")
        if runTests() {
            log.Println("Tests passed, creating pull request...")
            commitAndPush(data)
            log.Println("Changes pushed to branch.")
            createPullRequest(data)
            return
        }
        prompt += "\nTest failed, please address the following issues."
    }
    log.Println("Exceeded maximum attempts, please review manually.")
}

// All helper functions go here

// applyChangesWithChatGPT sends a prompt to ChatGPT, retrieves the response, and applies any changes
// specified in the response to the relevant files in the local repository.
func applyChangesWithChatGPT(data types.FormData, prompt string) error {
    // Create a ChatGPT request with the initial prompt
    request := chatgpt.CreateRequest(prompt)

    // Send the request to ChatGPT and get a response
    response, err := chatgpt.SendRequest(request)
    if err != nil {
        return fmt.Errorf("failed to get response from ChatGPT: %v", err)
    }

    // Parse the response to extract file contents based on delimiters
    filesContent, success := parseResponseForFiles(response)
    if !success {
        return fmt.Errorf("failed to parse files from ChatGPT response")
    }

    // Loop through each file path and content pair, writing the content to the specified file path
    for filePath, content := range filesContent {
        err := ioutil.WriteFile(filePath, []byte(content), 0644)
        if err != nil {
            log.Printf("failed to write changes to file %s: %v", filePath, err)
            continue
        }
        log.Printf("Successfully applied changes to %s", filePath)
    }

    return nil
}

// parseResponseForFiles extracts the content for each file based on the START and END delimiters.
// It returns a map where the keys are the full file paths and the values are the corresponding file contents.
// Additionally, it returns a boolean indicating success (true if any files were parsed successfully).
func parseResponseForFiles(response string) (map[string]string, bool) {
    filesContent := make(map[string]string)

    // Regex to match the START delimiter with the filename
    startRegex := regexp.MustCompile(`/\* START OF FILE: (.*?) \*/`)
    endRegex := regexp.MustCompile(`/\* END OF FILE: .*? \*/`)

    // Find all start matches and iterate over them
    startMatches := startRegex.FindAllStringSubmatchIndex(response, -1)
    if len(startMatches) == 0 {
        return nil, false // No files found
    }

    for _, startMatch := range startMatches {
        //start := startMatch[0]
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

    return filesContent, true
}

func cloneAndCheckoutRepo(data types.FormData) error {
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

func commitAndPush(data types.FormData) error {
    cmd := exec.Command("git", "add", ".")
    cmd.Dir = "repo"
    if err := cmd.Run(); err != nil {
        return err
    }

    cmd = exec.Command("git", "commit", "-m", "Applying requested changes")
    cmd.Dir = "repo"
    if err := cmd.Run(); err != nil {
        log.Println("Failed to commit the changes.")
        return err
    }

    log.Println("Changes committed, pushing to branch...")
    cmd = exec.Command("git", "push", "-u", "origin", data.Branch)
    cmd.Dir = "repo"
    return cmd.Run()
}

func createPullRequest(data types.FormData) {
    cmd := exec.Command("gh", "pr", "create", "--title", "Automated Changes", "--body", "Please review the automated changes.")
    cmd.Dir = "repo"
    cmd.Run()
}

func runTests() bool {
    cmd := exec.Command("make", "tests")
    cmd.Dir = "repo"
    err := cmd.Run()
    return err == nil
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



