package tag

import (
	"encoding/json"
	"strings"
	"sync"
)

type TagClickResponse struct {
	SelectedTags []string          `json:"selectedTags"`
	SetStorage   map[string]string `json:"setStorage"`
}

var (
	clickCounts       = make(map[string]int)
	isSelecting       = false
	selectedLetters   = make([]string, 0)
	selectedTags      = make([]string, 0)
	lastClickedLetter string
	consecutiveClicks int
	consecutiveCount  = 3
	mu                sync.Mutex
)

func Click(tag string) string {
	mu.Lock()
	defer mu.Unlock()

	letter := strings.ToLower(string(tag[0]))

	if _, exists := clickCounts[letter]; !exists {
		clickCounts[letter] = 0
	}

	if letter == lastClickedLetter {
		consecutiveClicks++
	} else {
		consecutiveClicks = 1
		lastClickedLetter = letter
	}

	storage := map[string]string{}
	if consecutiveClicks == consecutiveCount {
		if !isSelecting {
			isSelecting = true
			selectedLetters = make([]string, 0)
			selectedTags = make([]string, 0)
		} else {
			isSelecting = false

			letterString := strings.Join(selectedLetters[:len(selectedLetters)-consecutiveCount+1], "")
			storage["font"] = letterString

			selectedLetters = make([]string, 0)
			selectedTags = make([]string, 0)
			consecutiveClicks = 0
			lastClickedLetter = ""
		}
	} else if isSelecting {
		selectedLetters = append(selectedLetters, letter)
		selectedTags = append(selectedTags, tag)
	}

	clickCounts[letter]++

	response := TagClickResponse{
		SelectedTags: selectedTags,
		SetStorage:   storage,
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		return `{"error": "Failed to marshal response"}`
	}

	return string(jsonBytes)
}
