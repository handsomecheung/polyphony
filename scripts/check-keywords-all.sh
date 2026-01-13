#!/usr/bin/env bash
set -e

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

GIT_ROOT=$(git rev-parse --show-toplevel)
BLACKWORDS_FILE="$GIT_ROOT/infra/git/blackwords"

# Get all files in the project root directory (respecting .gitignore)
# --cached: include tracked files
# --others: include untracked files
# --exclude-standard: respect .gitignore rules
FILES=$(git ls-files --cached --others --exclude-standard)

if ! check_files_for_keywords "${BLACKWORDS_FILE}" "$FILES"; then
    echo -e "${RED}Check failed!${NC}"
    echo "Please remove the sensitive keywords from your files."
    exit 1
fi

exit 0
