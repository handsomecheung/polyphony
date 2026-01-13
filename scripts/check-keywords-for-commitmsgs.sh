#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

GIT_ROOT=$(git rev-parse --show-toplevel)
BLACKWORDS_FILE="$GIT_ROOT/infra/git/blackwords"

echo "Checking commit messages for sensitive keywords..."

FOUND=0
TEMP_MSG_FILE=$(mktemp)

trap 'rm -f "$TEMP_MSG_FILE"' EXIT

while IFS= read -r commit_sha; do
    git log -1 --format=%B "$commit_sha" >"$TEMP_MSG_FILE"

    CHECK_OUTPUT=$(check_commit_message_for_keywords "$BLACKWORDS_FILE" "$TEMP_MSG_FILE" 2>&1)
    CHECK_STATUS=$?

    if [ $CHECK_STATUS -ne 0 ]; then
        if [ $FOUND -eq 0 ]; then
            echo -e "${RED}ERROR: Sensitive keywords found in commit messages!${NC}"
            echo ""
            FOUND=1
        fi
        echo -e "${YELLOW}  Commit: $commit_sha${NC}"
        echo -e "  Subject: $(git log -1 --format="%s" "$commit_sha")${NC}"
        echo "$CHECK_OUTPUT" | tail -n +2
        echo ""
    fi
done < <(git rev-list --all)

if [ $FOUND -eq 1 ]; then
    echo -e "${RED}Check failed!${NC}"
    echo "Please review the commit messages containing sensitive keywords."
    exit 1
fi

echo "No sensitive keywords found in commit messages."
exit 0
