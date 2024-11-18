package assistant

import (
    "testing"
    "github.com/thomasdullien/coding-assistant/assistant/types"
)

// Test buildPrompt function
func TestBuildPrompt(t *testing.T) {
    userPrompt := "Add a new feature"
    deps := []string{"repo/file1.go", "repo/file2.go"}
    expected := "Add a new feature\n\nDependencies:\n\n/* START OF FILE: repo/file1.go */\n\n" +
                "Content of file1\n\n/* END OF FILE: repo/file1.go */\n\n" +
                "/* START OF FILE: repo/file2.go */\n\n" +
                "Content of file2\n\n/* END OF FILE: repo/file2.go */\n\n"
    
    // Mocking the file read operation, replace with actual file content for testing
    // This part would typically use a mocking library or test files for I/O operations
    fileContents := map[string]string{
        "repo/file1.go": "Content of file1",
        "repo/file2.go": "Content of file2",
    }

    originalReadFile := readFile // Capture the original function
    readFile = func(path string) ([]byte, error) {
        return []byte(fileContents[path]), nil
    }
    defer func() { readFile = originalReadFile }() // Restore original function

    got := buildPrompt(userPrompt, deps)
    if got != expected {
        t.Errorf("buildPrompt() = %v, want %v", got, expected)
    }
}

// Test calculateDependencies function
func TestCalculateDependencies(t *testing.T) {
    files := []string{"file1.cpp", "file2.cpp"}
    expected := []string{"repo/file1.cpp", "repo/file2.cpp"} // Expected output with repo/ prepended
    
    got, err := calculateDependencies(files)
    if err != nil {
        t.Fatalf("calculateDependencies() error = %v", err)
    }

    for i, dep := range got {
        if dep != expected[i] {
            t.Errorf("calculateDependencies() got = %v, want %v", got, expected)
        }
    }
}

// Test includeEntireRepo function
func TestIncludeEntireRepo(t *testing.T) {
    repoPath := "repo"
    
    // Mocking filesystem interactions would typically involve using a library such as `os` and `ioutil`.
    expectedFilesCount := 2 // Expected number of .go files, adjust as per test setup

    files, err := includeEntireRepo(repoPath)
    if err != nil {
        t.Fatalf("includeEntireRepo() error = %v", err)
    }

    if len(files) != expectedFilesCount {
        t.Errorf("includeEntireRepo() got %d files, want %d", len(files), expectedFilesCount)
    }
}

// Test ProcessAssistant function behavior (integration type)
func TestProcessAssistant(t *testing.T) {
    data := types.FormData{
        GithubUser: "testuser",
        RepoURL: "https://github.com/testuser/repo.git",
        Branch: "assistant-branch",
        Files: []string{"file1.go", "file2.go"},
        Prompt: "Test Prompt",
        RepoType: "Golang", // Test with Golang type
    }
    
    prLink, err := ProcessAssistant(data)
    if err != nil {
        t.Fatalf("ProcessAssistant() error = %v", err)
    }

    // Assuming the PR link ends with 'pulls' and contains 'testuser' 
    if prLink == "" || !contains(prLink, "pulls") {
        t.Errorf("Expected a valid PR link, got: %s", prLink)
    }
}

// Helper function to determine if a string contains another
func contains(s, substr string) bool {
    return strings.Contains(s, substr)
}