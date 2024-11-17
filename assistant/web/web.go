package web

import (
    "text/template"
    "net/http"
    "log"

    "github.com/thomasdullien/coding-assistant/assistant/assistant"
    "github.com/thomasdullien/coding-assistant/assistant/types" 
)

var tmpl = template.Must(template.ParseFiles("web/templates/index.html"))
var resultTmpl = template.Must(template.ParseFiles("web/templates/result.html"))

// Serve the web interface
func ServeWebInterface() {
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/submit", submitHandler)
    http.ListenAndServe(":8080", nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    tmpl.Execute(w, nil)
}

// Handle form submission
func submitHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    data := types.FormData{
        GithubUser:   r.FormValue("githubUser"),
        RepoURL:      r.FormValue("repoURL"),
        Branch:       "assistant-branch",
        Files:        r.Form["files"],
        Prompt:       r.FormValue("prompt"),
        RepoType:     r.FormValue("repoType"), // Capture the repository type
    }

    // Run ProcessAssistant and capture the pull request link or error
    prLink, err := assistant.ProcessAssistant(data)
    if err != nil {
        log.Printf("Error in ProcessAssistant: %v", err)
        resultTmpl.Execute(w, map[string]string{
            "Message": "An error occurred: " + err.Error(),
            "Link":    "",
        })
        return
    }

    // Show the result page with the pull request link
    resultTmpl.Execute(w, map[string]string{
        "Message": "Pull request created successfully!",
        "Link":    prLink,
    })
}


