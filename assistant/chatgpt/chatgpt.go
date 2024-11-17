```go
package chatgpt

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
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

const systemprompt = "You are an expert C++ developer assistant. Please execute the task described below..."

func CreateRequest(prompt string) ChatGPTRequest {
    return ChatGPTRequest{
        Model: "gpt-4o-mini",
        Messages: []Message{
            {Role: "system", Content: systemprompt},
            {Role: "user", Content: prompt},
        },
    }
}

func SendRequest(request ChatGPTRequest) (string, error) {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
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

    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    log.Printf("ChatGPT request: %s", string(requestBody))
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
        return "", err
    }

    if len(chatResponse.Choices) > 0 {
        return chatResponse.Choices[0].Message.Content, nil
    }

    return "", fmt.Errorf("no response from ChatGPT")
}
```