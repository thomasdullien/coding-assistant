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
'/* START OF FILE: $filename */' and '