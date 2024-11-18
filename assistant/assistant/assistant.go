package assistant

import (
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "strings"
    "bytes"
    "bufio"
    "path/filepath"
    "time" // Import time package

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

        log.Println("Running build...")
        builderr, buildout := runTestsOrBuild(data.RepoType, true)
        if !builderr {
          log.Println("Build successful.")
        } else {
          prompt += "\nBuild failed, please address the following issues:\n" + buildout
          continue
        }

        // Run tests and create pull request if successful
        log.Println("Running tests...")
        // For the moment, assume that Golang tests always pass. This
        // needs to change in the future.
        testerr, output := runTestsOrBuild(data.RepoType, false)

        if !testerr {
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
        } else {
          prompt += "\nTest failed, please address the following issues:\n" + output
        }            
    }
    log.Println("Exceeded maximum attempts, please review manually.")
    return "", fmt.Errorf("Exceeded maximum attempts to fix the test, please review.")
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
      data.Branch = fmt.Sprintf("assistant-%s-%s", summary, time.Now().Format("20060102150405")) // Update branch name with timestamp
      if err != nil {
        log.Fatalf("Error renaming branch: %v", err)
      }
    }

    // Loop through each file path and content pair
    for filePath, newContent := range filesContent {
        if strings.Contains(newContent, "\n// ... remaining functions unchanged") {
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

func runTestsOrBuild(repoType string, isBuild bool) (bool, string) {
  var cmd *exec.Cmd
  var action string
  if isBuild {
    action = "Build"
  } else {
    action = "Test"
  }

  if repoType == "C++" && isBuild {
    cmd = exec.Command("make", "build")
  } else if repoType == "C++" && !isBuild {
    cmd = exec.Command("make", "tests")
  } else if repoType == "Golang" && isBuild {
    cmd = exec.Command("go", "build", "-o", "build-out-executable", ".")
  } else if repoType == "Golang" && !isBuild {
    cmd = exec.Command("go", "test", "./...")
  } else {
    return false, "Unknown repository type"
  }
  // Set the working directory to "repo"
  cmd.Dir = "repo"

  // Capture stdout and stderr
  var outBuf, errBuf bytes.Buffer
  cmd.Stdout = &outBuf
  cmd.Stderr = &errBuf

  // Run the command
  err := cmd.Run()

  // Combine stdout and stderr for logging or further prompting
  output := outBuf.String() + "\n" + errBuf.String()

  if err != nil {
      // Log the failure and output
      log.Printf("%s failed. Output:\n%s", action, output)
      return false, output
  }

  // Log success and return
  log.Printf("%s passed successfully.", action)
  os.Remove("build-out-executable")
  return true, output
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
