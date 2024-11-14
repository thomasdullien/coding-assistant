package web

import (
    "text/template"
    "net/http"

    "github.com/thomasdullien/coding-assistant/assistant/assistant"
    "github.com/thomasdullien/coding-assistant/assistant/types" 
)

var tmpl = template.Must(template.ParseFiles("web/templates/index.html"))

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
    }

    go assistant.ProcessAssistant(data)  // Call ProcessAssistant in the assistant package

    http.Redirect(w, r, "/", http.StatusSeeOther)
}


