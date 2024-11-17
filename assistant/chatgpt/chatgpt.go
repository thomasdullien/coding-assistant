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

const systemprompt = `You are an expert C++ developer assistant. 
Please execute the task described below. When replying, please reply with 
entire .cpp or .hpp files, not just the changes. Delimit the files with 
'/* START OF FILE: $filename */' and '/* END OF FILE: $filename */'. Please 
also include a three-word summary of the PR changes. The summary should be in 
the format 'Summary: $summary'. The summary should be a maximum of three words 
separated by dashes, and not include any other punctuation or special 
characters. It will be used to identify the branch name for the PR. Please 
provide a one-line commit message too, in the format 'Commit-Message: $message'.
Lastly, some coding guidelines:
  - Absolutely do not remove comments. It is OK to suggest improvements to
    comments.
  - Avoid large-scale deletions of code, unless specifically instructed. It is
    unlikely that large quantities of code need to be removed, so if you think
    that this is the case, odds are you misunderstood something.
  - It is unlikely that this systemprompt needs to be shortened significantly.
    You may suggest improvements to it, but removing large portions is probably
    going to deteriorate performance.

In your responses, if you need to skip unchanged code, use the following
string: '/* INSERT LINES $start-$end FROM $path/$filename */'. This will
allow me to re-assemble the full file with your changes.
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

