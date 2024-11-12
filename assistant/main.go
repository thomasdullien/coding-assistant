package main

import (
    "fmt"
    "assistant/web"
)

func main() {
    fmt.Println("Starting ASSISTANT...")
    web.ServeWebInterface()
}

