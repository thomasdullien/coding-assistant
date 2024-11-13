A coding assistant to assist with making changes to C++ code bases by providing PRs.

I do not like IDE integration for AI very much, and wanted to have something that can
provide PRs instead. I also have very limited amounts of time, so I decided to write
that thing largely by using ChatGPT.

This repository is the current state. You get a web interface on localhost where you
can provide your GitHub username, the (SSH) URL for the repository you want to edit,
the files that should be edited, and the prompt.

The code will check out the repository, calculate the `#include` dependencies, send
the file(s)-to-edit and their include dependencies to the OpenAI API, extract the
results, apply them, run `make tests`, commit them to a branch if tests pass, and
push the branch.

I still need to ask ChatGPT to add code to create the PR, and there are tons of other
things to still fix. That said, it has correctly submitted PRs for a hobby project
of mine.

For those curious about the process that created this code, the transcript of my
conversation with ChatGPT is here:

https://chatgpt.com/share/67352519-0dac-800f-8718-38efb309a3dd
