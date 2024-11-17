A coding assistant to assist with making changes to C++ and Golang code bases
by providing PRs via the GitHub and OpenAI API.

I do not like IDE integration for AI very much, and wanted to have something
that can provide PRs instead. I also have very limited amounts of time, so 
I decided to write that thing largely by using ChatGPT. The result is this
repository.

This repository is the current state. You get a web interface on localhost
where you can provide your GitHub username, the (SSH) URL for the repository
you want to edit, the files that should be edited, and the prompt.

The code will check out the repository, calculate the `#include` dependencies, 
send the file(s)-to-edit and their include dependencies to the OpenAI API,
extract the results, apply them, run `make tests`, commit them to a branch if 
tests pass, and push the branch.

I still need to ask ChatGPT to add code to create the PR, and there are tons
of other things to still fix. That said, it has correctly submitted PRs for 
a hobby project of mine.

For those curious about the process that created this code, the transcript of
my conversation with ChatGPT is here:

https://chatgpt.com/share/67352519-0dac-800f-8718-38efb309a3dd

Below a transcript of the first few messages of the conversation, giving ChatGPT
an overview of the goals of the project:

---- snip ----
Hey there. I would like to write a tool that helps me turn ChatGPT into a coding assistant, but not one that is integrated with my IDE, but rather one that is integrated into my GitHub workflow.

So I wish to write code for the following tool, which we call ASSISTANT.

1. I have a locally-running web interface with a prompt window as well as a few fields where I can provide access credentials for a GitHub user, and the name of a GitHub repository.
2. I fill in the credentials and the repository URL.
3. I write a prompt in which I describe a particular change that I wish to be done. I provide a list of C++ header or source files in which these changes need to be performed.
4. ASSISTANT checks out a local copy of the specified repository using the credentials provided in the web interface. It then creates a local branch of the code with a 
5. ASSISTANT calculates the header file dependencies for the files in which edits need to be transformed, possibly using some LLVM / Clang tooling.
6. ASSISTANT assembles all these files, and prepares a prompt to be submitted to ChatGPT. This prompt should include the prompt I provided in (3), but also the content of all the source files obtained in (5). The request to ChatGPT should ask ChatGPT to perform the changes described in (3). It then uses the ChatGPT API to request ChatGPT to perform the changes.
7. ASSISTANT waits for the results and applies the changes to the local repository.
8. ASSISTANT runs a local testsuite by running ‘make tests’.
9. If all tests pass, ASSISTANT commits the changes to the local branch, pushes it, uses the GitHub API (or CLI) to submit a PR to be reviewed.
10. If the local tests fails, ASSISTANT adds the test failures to the prompt and re-sends the request, up to 5 times. If after 5 such iterations no test passing can be achieved, it gives up and requests a human to take over.

I also want the tool to support Golang, in which case we will submit the entire
repo and skip the dependency calculation step.

