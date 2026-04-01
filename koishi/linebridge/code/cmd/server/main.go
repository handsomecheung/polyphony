package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/line/line-bot-sdk-go/v8/linebot"
	"gopkg.in/yaml.v3"
)

// --- Configuration Structs ---

type Config struct {
	Line             LineConfig                      `yaml:"line"`
	OpenClawDeviceID string                          `yaml:"openclaw_device_id"`
	OpenClaw         []map[string]OpenClawConfigItem `yaml:"openclaw"`
	Route            RouteConfig                     `yaml:"route"`
}

type LineConfig struct {
	User  map[string]string `yaml:"user"`
	Group map[string]string `yaml:"group"`
}

type OpenClawConfigItem struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

type RouteConfig struct {
	User  map[string]string `yaml:"user"`
	Group map[string]string `yaml:"group"`
}

// --- OpenClaw Client ---

type OpenClawFrame struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Event   string          `json:"event,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type OpenClawClient struct {
	name      string
	url       string
	token     string
	derivedID string
	pubKey    ed25519.PublicKey
	privKey   ed25519.PrivateKey
	savePath  string
	conn      *websocket.Conn
	mu        sync.Mutex
	sendChan  chan interface{}

	// Fields for Line callback
	lineBot   *linebot.Client
	shortToID map[string]string
}

func NewOpenClawClient(name, url, token, deviceID, savePath string, bot *linebot.Client, shortToID map[string]string) *OpenClawClient {
	seed := sha256.Sum256([]byte("seed:" + deviceID))
	priv := ed25519.NewKeyFromSeed(seed[:])
	pub := priv.Public().(ed25519.PublicKey)
	hID := sha256.Sum256(pub)
	derivedID := hex.EncodeToString(hID[:])

	return &OpenClawClient{
		name:      name,
		url:       url,
		token:     token,
		derivedID: derivedID,
		pubKey:    pub,
		privKey:   priv,
		savePath:  savePath,
		sendChan:  make(chan interface{}, 100),
		lineBot:   bot,
		shortToID: shortToID,
	}
}

func (c *OpenClawClient) Start() {
	go c.connectLoop()
	go c.sendLoop()
}

func (c *OpenClawClient) connectLoop() {
	for {
		log.Printf("[%s] Connecting to %s...", c.name, c.url)
		header := http.Header{}
		origin := c.url
		if strings.HasPrefix(origin, "wss://") {
			origin = "https://" + strings.TrimPrefix(origin, "wss://")
		} else if strings.HasPrefix(origin, "ws://") {
			origin = "http://" + strings.TrimPrefix(origin, "ws://")
		}
		header.Add("Origin", origin)

		conn, _, err := websocket.DefaultDialer.Dial(c.url, header)
		if err != nil {
			log.Printf("[%s] Connection failed: %v. Retrying...", c.name, err)
			time.Sleep(5 * time.Second)
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()
		log.Printf("[%s] Connected.", c.name)

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[%s] Connection lost: %v", c.name, err)
				break
			}
			c.handleIncoming(message)
		}

		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
		time.Sleep(2 * time.Second)
	}
}

func (c *OpenClawClient) handleIncoming(msg []byte) {
	var frame OpenClawFrame
	if err := json.Unmarshal(msg, &frame); err != nil {
		return
	}

	if frame.Event == "connect.challenge" {
		var challenge struct {
			Nonce string `json:"nonce"`
		}
		json.Unmarshal(frame.Payload, &challenge)

		clientID := "openclaw-control-ui"
		clientMode := "webchat"
		role := "operator"
		scopes := []string{"operator.read", "operator.write", "operator.admin", "operator.approvals", "operator.pairing"}
		scopesCsv := strings.Join(scopes, ",")
		signedAt := time.Now().UnixMilli()

		canonical := fmt.Sprintf("v2|%s|%s|%s|%s|%s|%d|%s|%s",
			c.derivedID, clientID, clientMode, role, scopesCsv, signedAt, c.token, challenge.Nonce)

		sigBytes := ed25519.Sign(c.privKey, []byte(canonical))
		signature := base64.RawURLEncoding.EncodeToString(sigBytes)

		connectParams := map[string]interface{}{
			"minProtocol": 3, "maxProtocol": 3,
			"client": map[string]interface{}{
				"id": clientID, "version": "1.0.0", "platform": "linux", "mode": clientMode,
			},
			"role": role, "scopes": scopes, "caps": []string{}, "commands": []string{},
			"permissions": map[string]interface{}{},
			"auth":        map[string]interface{}{"token": c.token},
			"locale":      "en-US", "userAgent": "openclaw-control-ui/1.0.0",
			"device": map[string]interface{}{
				"id": c.derivedID, "publicKey": base64.StdEncoding.EncodeToString(c.pubKey),
				"signature": signature, "signedAt": signedAt, "nonce": challenge.Nonce,
			},
		}
		paramsBytes, _ := json.Marshal(connectParams)
		c.Send(OpenClawFrame{Type: "req", ID: "init", Method: "connect", Params: json.RawMessage(paramsBytes)})
	}

	if frame.Event == "chat" {
		var payload struct {
			State      string `json:"state"`
			SessionKey string `json:"sessionKey"`
			Message    struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(frame.Payload, &payload); err == nil && payload.State == "final" {
			log.Printf("[%s] Received final message for session %s.", c.name, payload.SessionKey)
			c.saveFrameToFile(msg)

			// Route back to Line
			if strings.Contains(payload.SessionKey, "line-") {
				parts := strings.Split(payload.SessionKey, "line-")
				shortName := parts[len(parts)-1]
				if lineID, ok := c.shortToID[shortName]; ok {
					var responseText string
					for _, content := range payload.Message.Content {
						if content.Type == "text" {
							responseText += content.Text
						}
					}
					if responseText != "" {
						cleanedText := c.cleanMarkdown(responseText)
						if _, err := c.lineBot.PushMessage(lineID, linebot.NewTextMessage(cleanedText)).Do(); err != nil {
							log.Printf("[%s] Error pushing to Line (%s): %v", c.name, shortName, err)
						} else {
							log.Printf("[%s] Successfully pushed response to Line: %s", c.name, shortName)
						}
					}
				}
			}
		}
	}
}

func (c *OpenClawClient) cleanMarkdown(text string) string {
	// Simple stripping of markdown markers
	r := strings.NewReplacer("**", "", "__", "", "`", "", "~~", "")
	res := r.Replace(text)

	// Remove headers: # Title
	lines := strings.Split(res, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for i := 0; i < len(trimmed) && trimmed[i] == '#'; i++ {
				level++
			}
			cleanedLines = append(cleanedLines, strings.TrimSpace(trimmed[level:]))
		} else {
			cleanedLines = append(cleanedLines, line)
		}
	}
	return strings.Join(cleanedLines, "\n")
}

func (c *OpenClawClient) saveFrameToFile(data []byte) {
	filename := fmt.Sprintf("openclaw-%s-final-%d.json", c.name, time.Now().UnixNano())
	os.WriteFile(filepath.Join(c.savePath, filename), data, 0644)
}

func (c *OpenClawClient) Send(msg interface{}) {
	select {
	case c.sendChan <- msg:
	default:
	}
}

func (c *OpenClawClient) sendLoop() {
	for msg := range c.sendChan {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.WriteJSON(msg)
		}
		c.mu.Unlock()
	}
}

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	lineSecret := os.Getenv("LINE_CHANNEL_SECRET")
	lineToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if lineSecret == "" || lineToken == "" {
		log.Fatalf("FATAL: LINE_CHANNEL_SECRET or LINE_CHANNEL_ACCESS_TOKEN not set")
	}

	savePath := "/tmp/linebridge"
	os.MkdirAll(savePath, 0755)

	configData, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to read config file %s: %v", *configPath, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		log.Fatalf("FATAL: Failed to parse config file %s: %v", *configPath, err)
	}

	bot, _ := linebot.New(lineSecret, lineToken)

	// Prepare shortName to LineID mapping for reverse routing
	shortToID := make(map[string]string)
	for short, id := range cfg.Line.User {
		shortToID[short] = id
	}
	for short, id := range cfg.Line.Group {
		shortToID[short] = id
	}

	// Initialize OpenClaw Clients
	clients := make(map[string]*OpenClawClient)
	for _, m := range cfg.OpenClaw {
		for name, item := range m {
			client := NewOpenClawClient(name, item.URL, item.Token, cfg.OpenClawDeviceID, savePath, bot, shortToID)
			client.Start()
			clients[name] = client
		}
	}

	// Reverse lookup maps for forward routing
	userToShort := make(map[string]string)
	for short, id := range cfg.Line.User {
		userToShort[id] = short
	}
	groupToShort := make(map[string]string)
	for short, id := range cfg.Line.Group {
		groupToShort[id] = short
	}

	http.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		events, err := bot.ParseRequest(r)
		if err != nil {
			if err == linebot.ErrInvalidSignature {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			return
		}

		for _, event := range events {
			saveEventToFile(savePath, event)
			if event.Type == linebot.EventTypeMessage {
				handleMessageContent(bot, savePath, event.Message)
			}

			// Routing Logic
			var clientName string
			var senderShortName string
			if event.Source.Type == linebot.EventSourceTypeUser {
				senderShortName = userToShort[event.Source.UserID]
				clientName = cfg.Route.User[senderShortName]
			} else if event.Source.Type == linebot.EventSourceTypeGroup {
				senderShortName = groupToShort[event.Source.GroupID]
				clientName = cfg.Route.Group[senderShortName]
			}

			routed := false
			if oc, ok := clients[clientName]; ok {
				if msg, ok := event.Message.(*linebot.TextMessage); ok {
					reqID := fmt.Sprintf("line-%s-%d", event.WebhookEventID, time.Now().Unix())
					sessionKey := fmt.Sprintf("line-%s", senderShortName)

					chatReq := OpenClawFrame{
						Type:   "req",
						ID:     reqID,
						Method: "chat.send",
						Params: json.RawMessage(fmt.Sprintf(`{"message":%q,"sessionKey":"%s","idempotencyKey":"%s"}`, msg.Text, sessionKey, reqID)),
					}
					oc.Send(chatReq)
					log.Printf("Forwarded message text to OpenClaw [%s]: %s", clientName, senderShortName)
					routed = true
				}
			}

			if !routed && event.ReplyToken != "" {
				bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("月が綺麗ですね")).Do()
				log.Printf("No route or not a text message, sent default reply to %s", event.Source.UserID)
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting on :%s using config %s", port, *configPath)
	http.ListenAndServe(":"+port, nil)
}

func saveEventToFile(dir string, event *linebot.Event) error {
	data, _ := json.MarshalIndent(event, "", "  ")
	filename := fmt.Sprintf("event-%s-%d.json", event.WebhookEventID, time.Now().UnixNano())
	return os.WriteFile(filepath.Join(dir, filename), data, 0644)
}

func handleMessageContent(bot *linebot.Client, dir string, message linebot.Message) {
	var messageID string
	var originalFileName string

	switch m := message.(type) {
	case *linebot.ImageMessage:
		messageID = m.ID
	case *linebot.VideoMessage:
		messageID = m.ID
	case *linebot.AudioMessage:
		messageID = m.ID
	case *linebot.FileMessage:
		messageID = m.ID
		originalFileName = m.FileName
	default:
		return
	}

	res, err := bot.GetMessageContent(messageID).Do()
	if err != nil {
		return
	}
	defer res.Content.Close()

	var fileName string
	if originalFileName != "" {
		fileName = fmt.Sprintf("file-%s-%s", messageID, originalFileName)
	} else {
		exts, _ := mime.ExtensionsByType(res.ContentType)
		extension := ".bin"
		if len(exts) > 0 {
			extension = exts[0]
		}
		fileName = fmt.Sprintf("content-%s%s", messageID, extension)
	}

	out, _ := os.Create(filepath.Join(dir, fileName))
	defer out.Close()
	io.Copy(out, res.Content)
}
