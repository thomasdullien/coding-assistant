```go
package web

import (
    "log"
    "net/http"
    "text/template"

    "github.com/thomasdullien/coding-assistant/assistant/assistant"
    "github.com/thomasdullien/coding-assistant/assistant/types"
)

var tmpl = template.Must(template.ParseFiles("web/templates/index.html"))
var resultTmpl = template.Must(template.ParseFiles("web/templates/result.html"))

func ServeWebInterface() {
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/submit", submitHandler)
    http.ListenAndServe(":8080", nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
    tmpl.Execute(w, nil)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseForm(); err != nil {
        log.Printf("Error parsing form: %v", err)
        http.Error(w, "Unable to parse form.", http.StatusBadRequest)
        return
    }

    data := types.FormData{
        GithubUser:   r.FormValue("githubUser"),
        RepoURL:      r.FormValue("repoURL"),
        Branch:       "assistant-branch",
        Files:        r.Form["files"],
        Prompt:       r.FormValue("prompt"),
    }

    prLink, err := assistant.ProcessAssistant(data)
    if err != nil {
        log.Printf("Error in ProcessAssistant: %v", err)
        resultTmpl.Execute(w, map[string]string{
            "Message": "An error occurred: " + err.Error(),
            "Link":    "",
        })
        return
    }

    resultTmpl.Execute(w, map[string]string{
        "Message": "Pull request created successfully!",
        "Link":    prLink,
    })
}
```