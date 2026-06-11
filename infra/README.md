# infra

Git infrastructure and security hooks to prevent accidental commits of sensitive keywords or credentials. Implements pre-commit and commit-msg hooks that scan staged files and commit messages against a user-defined blacklist of sensitive terms before allowing commits to proceed.

## Overview

The `infra` directory provides a layered approach to secret management at the git level. Two hooks work in tandem to catch sensitive keywords before they reach the repository:

1. **pre-commit hook**: Scans staged files for sensitive keywords in both filenames and file content.
2. **commit-msg hook**: Scans commit messages for sensitive keywords.

Both hooks read from a user-configurable blacklist file (`infra/git/blackwords`) that must be created on a per-repository basis. This allows each developer to define their own set of sensitive terms relevant to their workflow.

## Components

### git/hooks/pre-commit

A bash hook that runs before git commits are created. It:

- Retrieves all staged files (added, copied, or modified) using `git diff --cached`.
- Loads the blacklist from `infra/git/blackwords`.
- Checks each staged file's filename for blacklisted keywords (case-insensitive).
- Checks each staged file's content for blacklisted keywords (case-insensitive).
- Rejects the commit with a clear error message if any keywords are found.
- Ignores symlinks and deleted files.

### git/hooks/commit-msg

A bash hook that runs before git commits are finalized. It:

- Reads the commit message from the temporary file provided by git.
- Loads the blacklist from `infra/git/blackwords`.
- Checks the commit message for blacklisted keywords (case-insensitive).
- Rejects the commit if any sensitive keywords are found in the message text.

### git/blackwords.sample

A template file showing the format of the blackwords list:

```
sensitive sample word 1
sensitive sample word 2
sensitive sample word 3
```

Each keyword must appear on its own line. Comments (lines starting with `#`) are ignored, as are empty lines.

### scripts/common.sh (repo root)

A shared utility library located at the repository root (`scripts/common.sh`), sourced by the hooks via `../../../scripts/common.sh` — it does **not** live inside `infra/`. It provides three reusable functions:

- **load_blackfile()**: Loads keywords from the blackwords file, handling comments and empty lines. Returns an error if the file does not exist or contains no keywords.
- **check_files_for_keywords()**: Inspects staged files against the blacklist. Reports which files contain keywords and in which part (filename or content). Shows up to 3 matching lines for each keyword found.
- **check_commit_message_for_keywords()**: Inspects commit messages against the blacklist. Reports matching keywords and the lines where they appear.

All functions provide color-coded output (red for errors, yellow for file names) for easy visibility in the terminal.

## Setup

After cloning the repository, configure git to use these hooks:

```bash
git config core.hooksPath infra/git/hooks
```

Then create your own blackwords file based on the sample:

```bash
cp infra/git/blackwords.sample infra/git/blackwords
# Edit infra/git/blackwords and add your project-specific sensitive keywords
```

The `.gitignore` file in `infra/git/` ensures that `infra/git/blackwords` is never accidentally committed to the repository, protecting your specific keyword list.

## Usage

Once configured, the hooks run automatically:

1. **During git add / git commit**: The pre-commit hook runs and checks all staged files.
2. **During git commit**: After the editor closes, the commit-msg hook runs and checks the message.

If a keyword is detected, git will abort the commit and display the exact files and lines containing the offending keyword. Simply remove the keyword and try again.

## Keyword Matching

Matching is case-insensitive and uses word boundaries via `grep -iw`, ensuring that:

- `password` will match `PASSWORD`, `Password`, `PASSWORD123` (no, last one won't match due to word boundary), etc.
- `password` in a file named `update_password.txt` will be caught.
- `password` embedded in a larger word like `mypassword` will **not** be caught (word boundary protection).

## Related Documentation

- Parent project: [polyphony](../README.md)
- Koishi cluster: [koishi/README.md](../koishi/README.md)
