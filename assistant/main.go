package main

import (
    "fmt"
    "github.com/thomasdullien/coding-assistant/assistant/web"
)

func main() {
    fmt.Println("Starting ASSISTANT...")
    web.ServeWebInterface()
}

