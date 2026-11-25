# Issue tracker: GitHub

Issues and specs for this repo live in GitHub Issues at `handsomecheung/polyphony`. Use `gh` from this repository.

## Conventions

- Create: `gh issue create --title "..." --body-file <file>`.
- Read: `gh issue view <number> --json number,title,body,labels,comments`.
- List: `gh issue list --state open --json number,title,body,labels,comments`.
- Comment: `gh issue comment <number> --body "..."`.
- Label: `gh issue edit <number> --add-label "..."` or `--remove-label "..."`.
- Close: comment with the reason, then run `gh issue close <number>`.
- Before the first issue creation in a session, verify the target with `gh repo view --json nameWithOwner`. File issues only in `handsomecheung/polyphony`.

## Pull requests as a triage surface

**PRs as a request surface: no.**

## Skill terminology

- “Publish to the issue tracker” means create a GitHub issue.
- “Fetch the relevant ticket” means read its GitHub issue.
