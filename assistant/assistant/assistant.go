package assistant

import (
    "fmt"
    "io/ioutil"
    "log"
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
    err := cloneAndCheckoutRepo(data)
    if err != nil {
        log.Fatalf("Failed to clone repository: %v", err)
    }

    // Calculate dependencies
    deps, err := calculateDependencies(data.Files)
    if err != nil {
        log.Fatalf("Failed to calculate dependencies: %v", err)
    }

    // Prepare prompt
    prompt := buildPrompt(data.Prompt, deps)

    // Query ChatGPT and apply changes iteratively
    for attempts := 0; attempts < 5; attempts++ {
        err := applyChangesWithChatGPT(data, prompt)
        if err != nil {
            log.Fatalf("Failed to apply changes: %v", err)
        }

        // Run tests
        if runTests() {
            commitAndPush(data)
            createPullRequest(data)
            return
        }
        prompt += "\nTest failed, please address the following issues."
    }
    log.Println("Exceeded maximum attempts, please review manually.")
}

// All helper functions go here

func applyChangesWithChatGPT(data types.FormData, prompt string) error {
    request := chatgpt.CreateRequest(prompt)
    response, err := chatgpt.SendRequest(request)
    if err != nil {
        return fmt.Errorf("failed to get response from ChatGPT: %v", err)
    }

    fmt.Println("ChatGPT response:", response)
    for _, file := range data.Files {
        if changes, ok := parseResponseForFile(response, file); ok {
            err := ioutil.WriteFile("repo/"+file, []byte(changes), 0644)
            if err != nil {
                return fmt.Errorf("failed to write changes to file %s: %v", file, err)
            }
        } else {
            log.Printf("No changes for file %s", file)
        }
    }

    return nil
}

func parseResponseForFile(response, filename string) (string, bool) {
    sections := strings.Split(response, "filename:")
    for _, section := range sections {
        lines := strings.Split(section, "\n")
        if len(lines) > 1 && strings.TrimSpace(lines[0]) == filename {
            return strings.Join(lines[1:], "\n"), true
        }
    }
    return "", false
}

func cloneAndCheckoutRepo(data types.FormData) error {
    cmd := exec.Command("git", "clone", data.RepoURL, "repo")

    // Capture stdout and stderr
    var outBuf, errBuf bytes.Buffer
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf

    if err := cmd.Run(); err != nil {
        log.Printf("Failed to clone repository: %v", err)
        log.Printf("Output: %s", outBuf.String())
        log.Printf("StdErr: %s", errBuf.String())
        return err
    }

    cmd = exec.Command("git", "checkout", "-b", data.Branch)
    cmd.Dir = "repo"
    return cmd.Run()
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
        return err
    }

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

func buildPrompt(userPrompt string, deps []string) string {
    return userPrompt + "\nDependencies:\n" + strings.Join(deps, "\n")
}



