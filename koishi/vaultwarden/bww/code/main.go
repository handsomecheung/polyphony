package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Item struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
	Login  struct {
		Password string `json:"password"`
	} `json:"login"`
}

var (
	cache      = make(map[string]Item)
	cacheMutex sync.RWMutex
)

type BWStatus struct {
	Status string `json:"status"`
}

func ensureUnlocked() error {
	output, _ := exec.Command("bw", "status").CombinedOutput()
	var status BWStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return fmt.Errorf("failed to parse bw status: %v, output: %s", err, string(output))
	}

	switch status.Status {
	case "unlocked":
		return nil
	case "unauthenticated":
		log.Println("BW is unauthenticated, logging in...")
		if output, err := exec.Command("bw", "login", "--apikey").CombinedOutput(); err != nil {
			return fmt.Errorf("login failed: %s, %v", string(output), err)
		}
		fallthrough
	case "locked":
		log.Println("BW is locked, unlocking...")
		output, err := exec.Command("bw", "unlock", "--passwordenv", "BW_PASSWORD", "--raw").CombinedOutput()
		if err != nil {
			return fmt.Errorf("unlock failed: %s, %v", string(output), err)
		}
		session := strings.TrimSpace(string(output))
		os.Setenv("BW_SESSION", session)
		log.Println("BW unlocked and session updated.")
		return nil
	default:
		return fmt.Errorf("unexpected bw status: %s", status.Status)
	}
}

func bwCommand(args ...string) *exec.Cmd {
	session := os.Getenv("BW_SESSION")
	if session != "" {
		args = append(args, "--session", session)
	}
	return exec.Command("bw", args...)
}

func syncCache() error {
	if err := ensureUnlocked(); err != nil {
		return err
	}

	log.Println("Syncing with server...")
	if output, err := bwCommand("sync").CombinedOutput(); err != nil {
		return fmt.Errorf("sync failed: %s, %v", string(output), err)
	}

	log.Println("Fetching items...")
	output, err := bwCommand("list", "items").CombinedOutput()
	if err != nil {
		return fmt.Errorf("list items failed: %s, %v", string(output), err)
	}

	var items []Item
	if err := json.Unmarshal(output, &items); err != nil {
		return fmt.Errorf("failed to parse items: %v", err)
	}

	newCache := make(map[string]Item)
	for _, item := range items {
		newCache[item.Name] = item
	}

	cacheMutex.Lock()
	cache = newCache
	cacheMutex.Unlock()

	log.Printf("Loaded %d items into memory.", len(items))
	return nil
}

func getItem(name string) (Item, bool) {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	item, ok := cache[name]
	return item, ok
}

func getPasswordValue(name string) (string, error) {
	log.Printf("Fetching password for item: %s", name)

	item, ok := getItem(name)
	if !ok {
		return "", fmt.Errorf("item not found")
	}
	return item.Login.Password, nil
}

func getFieldValue(name, fieldName string) (string, error) {
	log.Printf("Fetching field '%s' for item: %s", fieldName, name)

	item, ok := getItem(name)
	if !ok {
		return "", fmt.Errorf("item not found")
	}
	for _, f := range item.Fields {
		if f.Name == fieldName {
			return f.Value, nil
		}
	}
	return "", fmt.Errorf("field not found")
}

func getAttachmentValue(name, filename string, isBase64 bool) ([]byte, error) {
	log.Printf("Fetching attachment '%s' for item: %s (base64: %v)", filename, name, isBase64)

	if err := ensureUnlocked(); err != nil {
		return nil, err
	}

	item, ok := getItem(name)
	if !ok {
		return nil, fmt.Errorf("item not found")
	}

	tmpFile, err := os.CreateTemp("", "bw-attach-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cmd := bwCommand("get", "attachment", filename, "--itemid", item.ID, "--output", tmpPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s, %v", string(output), err)
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}

	if isBase64 {
		return []byte(base64.StdEncoding.EncodeToString(content)), nil
	}
	return content, nil
}

func handleSync(w http.ResponseWriter, r *http.Request) {
	log.Println("Received sync request")
	if r.Method != "UPDATE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := syncCache(); err != nil {
		log.Printf("Sync failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Sync successful"))
}

var rePlaceholder = regexp.MustCompile(`__\{\{(.+?)\}\}__`)

func handleRender(w http.ResponseWriter, r *http.Request) {
	log.Println("Received render request")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read render body: %v", err)
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	result := rePlaceholder.ReplaceAllFunc(body, func(match []byte) []byte {
		content := string(match[4 : len(match)-4])
		parts := strings.Split(content, ":")

		var val string
		var err error

		switch len(parts) {
		case 1:
			val, err = getPasswordValue(parts[0])
		case 3:
			if parts[1] == "f" {
				val, err = getFieldValue(parts[0], parts[2])
			} else if parts[1] == "a" {
				var bVal []byte
				bVal, err = getAttachmentValue(parts[0], parts[2], false)
				val = string(bVal)
			}
		case 5:
			if parts[1] == "a" && parts[3] == "a" && parts[4] == "b64" {
				var bVal []byte
				bVal, err = getAttachmentValue(parts[0], parts[2], true)
				val = string(bVal)
			}
		}

		if err != nil {
			log.Printf("Render error for %s: %v", content, err)
			return match
		}
		return []byte(val)
	})

	w.Write(result)
}

func handlePassword(w http.ResponseWriter, r *http.Request) {
	name := strings.Split(r.URL.Path, "/")[1]
	val, err := getPasswordValue(name)
	if err != nil {
		log.Printf("Failed to get password for %s: %v", name, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Write([]byte(val))
}

func handleField(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	name := parts[1]
	fieldName := parts[3]
	val, err := getFieldValue(name, fieldName)
	if err != nil {
		log.Printf("Failed to get field '%s' for %s: %v", fieldName, name, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Write([]byte(val))
}

func handleAttachment(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	name := parts[1]
	fileName := parts[3]
	isBase64 := len(parts) > 4 && parts[4] == "base64"
	val, err := getAttachmentValue(name, fileName, isBase64)
	if err != nil {
		log.Printf("Failed to get attachment '%s' for %s: %v", fileName, name, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isBase64 {
		w.Header().Set("Content-Type", "text/plain")
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	}
	w.Write(val)
}

func main() {
	if err := syncCache(); err != nil {
		log.Fatalf("Initial sync failed: %v", err)
	}

	http.HandleFunc("/sync", handleSync)
	http.HandleFunc("/render", handleRender)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.NotFound(w, r)
			return
		}
		action := parts[2]
		switch action {
		case "password":
			handlePassword(w, r)
		case "field":
			handleField(w, r)
		case "attachment":
			handleAttachment(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	fmt.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
