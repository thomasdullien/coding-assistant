package web

import (
    "html/template"
    "net/http"
)

// Struct to hold form data
type FormData struct {
    GithubUser   string
    GithubToken  string
    RepoURL      string
    Branch       string
    Files        []string
    Prompt       string
}

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
    // Parse form
    r.ParseForm()
    data := FormData{
        GithubUser:   r.FormValue("githubUser"),
        GithubToken:  r.FormValue("githubToken"),
        RepoURL:      r.FormValue("repoURL"),
        Branch:       "assistant-branch",
        Files:        r.Form["files"],
        Prompt:       r.FormValue("prompt"),
    }

    // Start the process asynchronously
    go ProcessAssistant(data)

    // Display confirmation to the user
    http.Redirect(w, r, "/", http.StatusSeeOther)
}

