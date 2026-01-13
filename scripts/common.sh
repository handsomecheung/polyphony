#!/usr/bin/env bash
set -e

export RED='\033[0;31m'
export YELLOW='\033[1;33m'
export NC='\033[0m' # No Color

# Function to check files for sensitive keywords
# Parameters:
#   $1 - BLACKWORDS_FILE:
#   $2 - FILES: list of files to check (newline or space separated)
# Returns:
#   0 - no sensitive keywords found
#   1 - sensitive keywords found
check_files_for_keywords() {
    local BLACKWORDS_FILE="$1"
    local FILES="$2"
    local FOUND=0
    local KEYWORDS=()

    if [ ! -f "$BLACKWORDS_FILE" ]; then
        echo -e "${RED}ERROR: Blackwords file not found at $BLACKWORDS_FILE${NC}"
        echo "Please create the file and define sensitive keywords (one per line)."
        echo "Commit rejected!"
        return 1
    fi

    while IFS= read -r line; do
        # Skip empty lines and lines starting with #
        if [ -n "$line" ] && [[ ! "$line" =~ ^[[:space:]]*# ]]; then
            KEYWORDS+=("$line")
        fi
    done <"$BLACKWORDS_FILE"

    if [ ${#KEYWORDS[@]} -eq 0 ]; then
        echo -e "${RED}ERROR: No keywords defined in $BLACKWORDS_FILE${NC}"
        echo "The blackwords file is empty or contains only comments."
        echo "Please add sensitive keywords to the file (one per line)."
        echo "Commit rejected!"
        return 1
    fi

    if [ -z "$FILES" ]; then
        return 0
    fi

    echo "Checking for sensitive keywords..."

    for FILE in $FILES; do
        for KEYWORD in "${KEYWORDS[@]}"; do
            if echo "$FILE" | grep -qiw "$KEYWORD"; then
                if [ $FOUND -eq 0 ]; then
                    echo -e "${RED}ERROR: Sensitive keywords found in files!${NC}"
                    echo ""
                    FOUND=1
                fi
                echo -e "${YELLOW}  File: $FILE${NC}"
                echo -e "${RED}  Keyword found in FILENAME: $KEYWORD${NC}"
                echo ""
            fi
        done

        # Skip if file doesn't exist (deleted files)
        if [ ! -f "$FILE" ]; then
            continue
        fi

        for KEYWORD in "${KEYWORDS[@]}"; do
            if grep -qiw "$KEYWORD" "$FILE"; then
                if [ $FOUND -eq 0 ]; then
                    echo -e "${RED}ERROR: Sensitive keywords found in files!${NC}"
                    echo ""
                    FOUND=1
                fi
                echo -e "${YELLOW}  File: $FILE${NC}"
                echo -e "${RED}  Keyword found in CONTENT: $KEYWORD${NC}"
                # Show the line where keyword was found
                grep -niw "$KEYWORD" "$FILE" | head -n 3
                echo ""
            fi
        done
    done

    if [ $FOUND -eq 1 ]; then
        return 1
    fi

    echo "No sensitive keywords found."
    return 0
}
