package main

import (
    "fmt"
    "io/ioutil"
    "log"
    "os/exec"
    "strings"

    "assistant/web"
    "assistant/chatgpt"
)

func main() {
    fmt.Println("Starting ASSISTANT...")
    web.ServeWebInterface()
}

// ProcessAssistant handles the main workflow
func ProcessAssistant(data web.FormData) {
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

// Builds the prompt, sends to ChatGPT, and applies changes
func applyChangesWithChatGPT(data web.FormData, prompt string) error {
    // Create a ChatGPT request with the initial prompt
    request := chatgpt.CreateRequest(prompt)

    // Send the request to ChatGPT and get a response
    response, err := chatgpt.SendRequest(request)
    if err != nil {
        return fmt.Errorf("failed to get response from ChatGPT: %v", err)
    }

    // Process the response to identify code changes
    fmt.Println("ChatGPT response:", response)

    // This is a simplified example: you may need to adjust the parsing based on response format.
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

// Example of parsing the response to extract changes for each file
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

func cloneAndCheckoutRepo(data web.FormData) error {
    cmd := exec.Command("git", "clone", data.RepoURL, "repo")
    if err := cmd.Run(); err != nil {
        return err
    }

    cmd = exec.Command("git", "checkout", "-b", data.Branch)
    cmd.Dir = "repo"
    return cmd.Run()
}

func calculateDependencies(files []string) ([]string, error) {
    args := append([]string{"cpp-dependencies"}, files...)
    cmd := exec.Command(args[0], args[1:]...)
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    return strings.Split(string(output), "\n"), nil
}

func commitAndPush(data web.FormData) error {
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

func createPullRequest(data web.FormData) {
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

