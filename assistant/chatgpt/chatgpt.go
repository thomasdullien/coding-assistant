package chatgpt

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "os"
    "log"
)

const openAIEndpoint = "https://api.openai.com/v1/chat/completions"

type ChatGPTRequest struct {
    Model    string   `json:"model"`
    Messages []Message `json:"messages"`
}

type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ChatGPTResponse struct {
    Choices []struct {
        Message Message `json:"message"`
    } `json:"choices"`
}

const systemprompt = `You are an expert C++ and Golang developer assistant. 
Please execute the task described below with the following guidelines:

1. When replying, please reply with entire .cpp or .hpp files, not just the
   changes. 
2. Delimit the files with the following markers:
   - Start each file with '/* START OF FILE: $filename */' 
   - End each file with '/* END OF FILE: $filename */'
3. If parts of the file are unchanged, do not omit or summarize them. Instead, 
   include the entire file. *This is extremely important*.
5. Additionally, include the following:
   - A three-word summary of the PR changes in the format "Summary: $summary".
     The summary should be a maximum of three words separated by dashes, and
     not include any other punctuation or special characters.
   - A one-line commit message in the format "Commit-Message: $message"
6. Absolutely do not remove comments. It is OK to suggest improvements to
   comments.
7. Ensure that you never return two copies of the same file, each file should
   only be present once.

Please ensure your replies strictly adhere to these rules to avoid ambiguity
and issues in creating PRs out of your changes. This is very important.
`

// CreateRequest prepares the prompt request for ChatGPT
func CreateRequest(prompt string) ChatGPTRequest {
    return ChatGPTRequest{
        Model: "gpt-4o-mini",
        Messages: []Message{
            {Role: "system", Content: systemprompt},
            {Role: "user", Content: prompt},
        },
    }
}

// SendRequest sends the prompt to ChatGPT and retrieves the response
func SendRequest(request ChatGPTRequest) (string, error) {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        log.Printf("OPENAI_API_KEY environment variable is not set")
        return "", fmt.Errorf("OPENAI_API_KEY environment variable is not set")
    }

    requestBody, err := json.Marshal(request)
    if err != nil {
        return "", err
    }

    req, err := http.NewRequest("POST", openAIEndpoint, bytes.NewBuffer(requestBody))
    if err != nil {
        return "", err
    }
    log.Printf("Request: %v", req)
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    // Log the request for debugging.
    fmt.Println("ChatGPT request:", string(requestBody))
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := ioutil.ReadAll(resp.Body)
        return "", fmt.Errorf("ChatGPT API error: %s", string(body))
    }

    var chatResponse ChatGPTResponse
    err = json.NewDecoder(resp.Body).Decode(&chatResponse)
    if err != nil {
        log.Printf("Failed to decode response: %v", err)
        return "", err
    }
    log.Printf("ChatGPT response: %v", chatResponse)

    if len(chatResponse.Choices) > 0 {
        return chatResponse.Choices[0].Message.Content, nil
    }

    return "", fmt.Errorf("no response from ChatGPT")
}

