package types

// FormData holds the form data submitted by the user
type FormData struct {
    GithubUser   string
    RepoURL      string
    Branch       string
    Files        []string
    Prompt       string
    RepoType     string
}

