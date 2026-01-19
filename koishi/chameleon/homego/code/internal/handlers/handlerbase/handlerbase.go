package handlerbase

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"koishi/chameleon/homego/internal/auth"
	"koishi/chameleon/homego/internal/common"
	"koishi/chameleon/homego/internal/config"
	"koishi/chameleon/homego/internal/encryption"
	"koishi/chameleon/homego/internal/markdown"
	"koishi/chameleon/homego/internal/queue"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	cookieClientID  = "client_id"
	cookieSessionID = "session_id"

	idKeyHelp            = "h"
	idKeyMBArticleList   = "showMBArticleList"
	idPrefixGetMBArticle = "getMBArticle:"

	GollumHomeMDName = "Home.md"

	mbPlaceholderTextPrefix  = "placeholder-inserttext"
	mbPlaceholderImagePrefix = "placeholder-insertimage"
)

type HandlerBase struct {
	Config      *config.Config
	AuthService *auth.AuthService
}

func NewHandler(cfg *config.Config, authService *auth.AuthService) *HandlerBase {
	return &HandlerBase{
		Config:      cfg,
		AuthService: authService,
	}
}

func (h *HandlerBase) CheckAuth(c *gin.Context) bool {
	return h.AuthService.CheckAuth(h.GetSessionID(c), h.FetchClientID(c))
}

func (h *HandlerBase) HomePage(c *gin.Context) {
	title := "Learning X in a real way"
	c.HTML(200, "home.html", gin.H{
		"title": title,
	})
}

func (h *HandlerBase) ShowKeyPage(c *gin.Context) {
	isAuth := h.CheckAuth(c)
	if !isAuth {
		c.String(404, "404 page not found")
		return
	}

	c.HTML(200, "key.html", gin.H{})
}

func (h *HandlerBase) GenHandlerIndexPage(category, pathPrefix string) func(c *gin.Context) {
	return func(c *gin.Context) {
		isAuth := h.CheckAuth(c)

		articleInfos := h.GetArticleInfos(pathPrefix)

		title := fmt.Sprintf("Learning %s in a real way", category)
		c.HTML(200, "index.html", gin.H{
			"pathPrefix":   pathPrefix,
			"isAuth":       isAuth,
			"title":        title,
			"articleInfos": articleInfos,
		})
	}
}

func (h *HandlerBase) GenHandlerArticleRanksPage(category, pathPrefix string) func(c *gin.Context) {
	return func(c *gin.Context) {
		isAuth := h.CheckAuth(c)

		if isAuth {
			h.AuthService.LogoutAll()
		} else {
			do, msg := h.returnBusy()
			if do {
				c.String(503, msg)
				return
			}
		}

		articleInfos := h.GetArticleInfos(pathPrefix)
		for i := range articleInfos {
			articleInfos[i].ViewCount = h.calViewCount(isAuth, articleInfos[i].Title)
		}

		sort.Slice(articleInfos, func(i, j int) bool {
			return articleInfos[i].ViewCount > articleInfos[j].ViewCount
		})

		title := fmt.Sprintf("Learning %s in a real way", category)
		c.HTML(200, "index.html", gin.H{
			"pathPrefix":   pathPrefix,
			"isAuth":       isAuth,
			"title":        title,
			"articleInfos": articleInfos,
		})
	}
}

func (h *HandlerBase) GenHandlerShowArticlePage(pathArticles, pathTry, pathRanks string) func(c *gin.Context) {
	return func(c *gin.Context) {
		encodedFilename := c.Param("encname")
		filename, err := encryption.CipherDecode(encodedFilename)
		if err != nil {
			fmt.Println("decode filename error: ", err)
			c.String(404, "404 page not found")
			return
		}

		clientID := h.FetchClientID(c)
		isAuth := h.CheckAuth(c)

		id, err := h.DecodeID(c.Query("id"))
		if err != nil {
			fmt.Println("decodeID error: ", err)
			c.String(404, "404 page not found")
			return
		}

		mdHTML := h.GetArticleHTML(isAuth, pathArticles, filename, id)
		if mdHTML == "" {
			c.String(404, "404 page not found")
			return
		}

		if !isAuth {
			if id == "" {
				fmt.Println("index can not be empty if not authenticated")
				c.String(404, "404 page not found")
				return
			}

			if common.IsDigital(id) {
				sessionID, _ := h.AuthService.Authenticate(clientID, id)
				if sessionID != "" {
					h.SetSessionID(c, sessionID)
					isAuth = true
				}
			}
		}

		title := GetArticleName(filename)
		mbContent := h.GetMBContent(c, id)

		c.HTML(200, "article.html", gin.H{
			"title":     title,
			"mdHTML":    template.HTML(mdHTML),
			"mbContent": mbContent,
			"pathHome":  pathArticles,
			"pathTry":   addQueryParam(pathTry, "arg", "main", "article", title),
			"pathRanks": pathRanks,
			"viewCount": h.calViewCount(isAuth, title),
		})
	}
}

func (h *HandlerBase) calViewCount(isAuath bool, title string) int {
	if isAuath {
		return common.RandIntRange(50, 0.1)
	}

	epoch := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	hash := 0
	for _, r := range title {
		hash += int(r)
	}
	offsetDays := hash % 365
	articleEpoch := epoch.AddDate(0, 0, offsetDays)
	days := max(time.Since(articleEpoch).Hours()/24, 1)
	return int(30*math.Sqrt(days)) + hash%100
}

func addQueryParam(basePath string, params ...string) string {
	if len(params)%2 != 0 {
		return basePath
	}

	separator := "?"
	if strings.Contains(basePath, "?") {
		separator = "&"
	}

	var queryParts []string
	for i := 0; i < len(params); i += 2 {
		queryParts = append(queryParts, fmt.Sprintf("%s=%s", params[i], params[i+1]))
	}

	return basePath + separator + strings.Join(queryParts, "&")
}

func splitPathAndQuery(pathWithQuery string) (string, string) {
	parts := strings.SplitN(pathWithQuery, "?", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return pathWithQuery, ""
}

func (h *HandlerBase) setCookie(c *gin.Context, key, value string) {
	domain := c.Request.Host
	fmt.Println("set cookie for domain: ", domain)

	c.SetCookie(key, value, int(h.Config.CookieExpiry.Seconds()),
		"/", domain, h.Config.CookieSecure, true)
}

func (h *HandlerBase) genClientID() string {
	return uuid.New().String()
}

func (h *HandlerBase) FetchClientID(c *gin.Context) string {
	clientID, err := c.Cookie(cookieClientID)
	if err != nil {
		clientID = h.genClientID()
		h.setCookie(c, cookieClientID, clientID)
	}
	return clientID
}

func (h *HandlerBase) TrySetFakeSessionID(c *gin.Context) {
	sessionID := h.GetSessionID(c)
	if sessionID == "" {
		fakeSessionID, _ := h.AuthService.EncryptClientID(h.genClientID())
		h.setCookie(c, cookieSessionID, fakeSessionID)
	}
}

func (h *HandlerBase) GetSessionID(c *gin.Context) string {
	sessionID, _ := c.Cookie(cookieSessionID)
	return sessionID
}

func (h *HandlerBase) SetSessionID(c *gin.Context, sessionID string) {
	h.setCookie(c, cookieSessionID, sessionID)
}

func (h *HandlerBase) getMBContentHelp(c *gin.Context) string {
	currentPath := c.Request.URL.Path

	links := []string{
		fmt.Sprintf("[Help](%s?id=%s)", currentPath, encryption.CipherEncode(idKeyHelp)),
		fmt.Sprintf("[Show MB List](%s?id=%s)", currentPath, encryption.CipherEncode(idKeyMBArticleList)),
	}

	return common.String2MB(strings.Join(links, "\n\n\n"))
}

func (h *HandlerBase) getMBContentMBArticleList(c *gin.Context) string {
	entries, err := os.ReadDir(getMBDirPath())
	if err != nil {
		fmt.Println("Error reading MB directory:", err)
		return ""
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	currentPath := c.Request.URL.Path

	links := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") && entry.Name() != GollumHomeMDName {
			name := strings.TrimSuffix(entry.Name(), ".md")
			id := encryption.CipherEncode(idPrefixGetMBArticle + name)
			links = append(links, fmt.Sprintf("[%s](%s?id=%s)", name, currentPath, id))
		}
	}

	return common.String2MB(strings.Join(links, "\n\n\n"))
}

func (h *HandlerBase) getMBContentArticle(name string) string {
	return common.File2MB(getMBFilePath(name + ".md"))
}

func (h *HandlerBase) GetPlaceholderTextContentById(id string) string {
	if common.IsDigital(id) {
		placeholderFiles := h.getPlaceholderInsertTextEntries()
		idint, _ := strconv.Atoi(id)
		if filePath, found := placeholderFiles[idint]; found {
			return common.File2MB(filePath)
		}
	}

	return ""
}

func (h *HandlerBase) GetPlaceholderImageTempPathById(id string) string {
	if common.IsDigital(id) {
		placeholderFiles := h.getPlaceholderInsertImageEntries()
		idint, _ := strconv.Atoi(id)
		if filePath, found := placeholderFiles[idint]; found {
			sourceImage := "/homego/articles/images/ps1/01.png"
			targetImage := "/tmp/images/ps1/01.png"
			if !common.FileExists(targetImage) {
				common.CallLowkey(filePath, sourceImage, targetImage)
			}
			return targetImage
		}
	}

	return ""
}

func (h *HandlerBase) GetMBContent(c *gin.Context, id string) string {
	fmt.Println("id: ", id)

	if id == idKeyHelp {
		return h.getMBContentHelp(c)
	} else if id == idKeyMBArticleList {
		return h.getMBContentMBArticleList(c)
	} else if strings.HasPrefix(id, idPrefixGetMBArticle) {
		return h.getMBContentArticle(strings.TrimPrefix(id, idPrefixGetMBArticle))
	}

	return ""
}

func (h *HandlerBase) DecodeID(id string) (string, error) {
	if id == "" {
		return "", nil
	}
	if id == idKeyHelp {
		return idKeyHelp, nil
	}
	return encryption.CipherDecode(id)
}

func (h *HandlerBase) getArticleDir(pathPrefix string) string {
	return filepath.Join("articles", "articles", pathPrefix)
}

func (h *HandlerBase) getArticlePath(pathPrefix string, filename string) string {
	return filepath.Join(h.getArticleDir(pathPrefix), filename)
}

func (h *HandlerBase) GetArticleHTML(isAuth bool, pathPrefix string, filename string, id string) string {
	md := markdown.GetFileContent(h.getArticlePath(pathPrefix, filename))
	md = h.processPlaceholderInsertTextTail(md, id)
	md = h.processPlaceholderInsertImageTail(md, id)
	html := markdown.Content2HTML(md)
	return h.processPlaceholderInsertTextInner(html)
}

func (h *HandlerBase) getPlaceholderInsertEntries(prefix string) map[int]string {
	placeholderFiles := make(map[int]string)

	insertDir := "/homego/article-insert"
	entries, err := os.ReadDir(insertDir)
	if err != nil {
		log.Printf("Error reading article-insert directory: %v\n", err)
		return placeholderFiles
	}

	filePattern := regexp.MustCompile(fmt.Sprintf(`^%s_(\d+)$`, prefix))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := filePattern.FindStringSubmatch(entry.Name())
		if len(matches) >= 2 {
			number, _ := strconv.Atoi(matches[1])
			placeholderFiles[number] = filepath.Join(insertDir, entry.Name())
		}
	}

	return placeholderFiles
}

func (h *HandlerBase) getPlaceholderInsertTextEntries() map[int]string {
	return h.getPlaceholderInsertEntries(mbPlaceholderTextPrefix)
}

func (h *HandlerBase) getPlaceholderInsertImageEntries() map[int]string {
	return h.getPlaceholderInsertEntries(mbPlaceholderImagePrefix)
}

func (h *HandlerBase) processPlaceholderInsertTextTail(md, id string) string {
	mbPlaceholder := h.GetPlaceholderTextContentById(id)
	if mbPlaceholder != "" {
		md = md + fmt.Sprintf("\n\n\n```\n%s\n```", mbPlaceholder)
	}

	return md
}

func (h *HandlerBase) processPlaceholderInsertImageTail(md, id string) string {
	placeholderTempPath := h.GetPlaceholderImageTempPathById(id)
	if placeholderTempPath != "" {
		rpath := strings.Replace(placeholderTempPath, "/tmp/images/", "/timages/", 1)
		md = md + fmt.Sprintf("\n\n\n ![text](%s) \n\n\n", rpath)
	}

	return md
}

func (h *HandlerBase) processPlaceholderInsertTextInner(html string) string {
	placeholderTextFiles := h.getPlaceholderInsertTextEntries()

	for number, filePath := range placeholderTextFiles {
		placeholder := fmt.Sprintf("%s_%d", mbPlaceholderTextPrefix, number)
		if strings.Contains(html, placeholder) {
			encryptedContent := common.File2MB(filePath)
			html = strings.ReplaceAll(html, placeholder, encryptedContent)
			log.Printf("Replaced placeholder %s with encrypted content from %s\n", placeholder, filePath)
		}
	}

	return html
}

func (h *HandlerBase) GetArticleInfos(pathPrefix string) []articleInfo {
	entries, err := os.ReadDir(h.getArticleDir(pathPrefix))
	if err != nil {
		return nil
	}

	filenames := []string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") && entry.Name() != GollumHomeMDName {
			filenames = append(filenames, entry.Name())
		}
	}

	var fileInfos []fileInfo
	for _, filename := range filenames {
		fileInfos = append(fileInfos, genFileInfo(filename))
	}

	// Sort by priority (lower priority numbers come first)
	sort.Slice(fileInfos, func(i, j int) bool {
		if fileInfos[i].Priority == fileInfos[j].Priority {
			// shuffling for files with the same priority
			return rand.Float64() < 0.5
		}
		return fileInfos[i].Priority < fileInfos[j].Priority
	})

	var articleInfos []articleInfo
	for index, fileInfo := range fileInfos {
		articleInfos = append(articleInfos, genArticleInfo(index, fileInfo.Name))
	}

	return articleInfos
}

func (h *HandlerBase) Auth(c *gin.Context) {
	if h.CheckAuth(c) {
		c.Header("X-Forwarded-User", "authenticated")
		c.Status(200)
		return
	}

	domain := c.Request.Host
	fmt.Println("redirect to domain: ", domain)
	c.Redirect(302, fmt.Sprintf("https://%s", domain))
}

type fileInfo struct {
	Name     string
	Priority int
}

func genFileInfo(filename string) fileInfo {
	return fileInfo{
		Name:     filename,
		Priority: getPriority(filename),
	}
}

type articleInfo struct {
	Title           string
	Id              string
	EncodedFilename string
	ViewCount       int
}

func genArticleInfo(index int, filename string) articleInfo {
	return articleInfo{
		Title:           GetArticleName(filename),
		EncodedFilename: encryption.CipherEncode(filename),
		Id:              encryption.CipherEncode(fmt.Sprintf("%d", index)),
	}
}

func splitPriority(filename string) (int, string) {
	// Example: "001.article.md" -> 1, "article.md"

	defaultPriority := 999
	for i, r := range filename {
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '.' && i > 0 {
			priority, err := strconv.Atoi(filename[0:i])
			if err != nil {
				return defaultPriority, filename
			}
			return priority, filename[i+1:]
		}
		break
	}

	return defaultPriority, filename
}

func tryRemovePriority(filename string) string {
	_, nameWithoutPriority := splitPriority(filename)
	return nameWithoutPriority
}

func getPriority(filename string) int {
	priority, _ := splitPriority(filename)
	return priority
}

func GetArticleName(filename string) string {
	return strings.TrimSuffix(tryRemovePriority(filename), ".md")
}

func getMBDirPath() string {
	return filepath.Join("articles", "mb")
}

func getMBFilePath(filename string) string {
	return filepath.Join(getMBDirPath(), filename)
}

func (h *HandlerBase) doShowQueue(c *gin.Context, queueManager *queue.QueueManager, redirect string) bool {
	clientID := h.FetchClientID(c)

	originalPath := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		originalPath += "?" + c.Request.URL.RawQuery
	}
	canAccess := queueManager.CanAccess(clientID, originalPath)
	if canAccess {
		return false
	}

	c.Redirect(302, redirect)
	return true
}

func (h *HandlerBase) TryBase(c *gin.Context, doCheckQueue bool, qm *queue.QueueManager, targetHost, pathTry, pathRedirect string, wsProtocl string) {
	if doCheckQueue && h.doShowQueue(c, qm, pathRedirect) {
		return
	}

	targetURL, err := url.Parse(targetHost)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	clientID := h.FetchClientID(c)
	ctx, cancel := context.WithCancel(c.Request.Context())
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host

		pathTryPath, pathTryQuery := splitPathAndQuery(pathTry)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, pathTryPath)
		if pathTryQuery != "" && !strings.Contains(req.URL.RawQuery, pathTryQuery) {
			if req.URL.RawQuery == "" {
				req.URL.RawQuery = pathTryQuery
			} else {
				req.URL.RawQuery += "&" + pathTryQuery
			}
		}
		log.Printf("proxy request to %s/%s?%s", req.Host, req.URL.Path, req.URL.RawQuery)

		*req = *req.WithContext(ctx)
	}

	// Handle WebSocket upgrade and connection monitoring
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode == http.StatusSwitchingProtocols {
			if wsProtocl != "" {
				resp.Header.Set("Sec-WebSocket-Protocol", wsProtocl)
			}
			if doCheckQueue {
				qm.DoAfterConnected(c.Writer, c.Request, clientID, ctx, cancel)
			}
		}
		return nil
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func (h *HandlerBase) GenHandlerQueuePage(queueManager *queue.QueueManager, pathArticles, pathLeave, pathCheck string) func(c *gin.Context) {
	return func(c *gin.Context) {
		clientID := h.FetchClientID(c)
		h.TrySetFakeSessionID(c)

		vnumerator, vdenominator, vposition, vtotal, canAccess, userEntity := queueManager.HearbeatAndGetStatusForPage(clientID)

		if userEntity == nil {
			c.Redirect(302, pathArticles)
			return
		}

		if canAccess {
			c.Redirect(302, userEntity.PathRedirect)
			return
		}

		c.HTML(200, "queue.html", gin.H{
			"canAccess":    canAccess,
			"numerator":    vnumerator,
			"denominator":  vdenominator,
			"position":     vposition,
			"total":        vtotal,
			"pathArticles": pathArticles,
			"pathRedirect": userEntity.PathRedirect,
			"pathLeave":    pathLeave,
			"pathCheck":    pathCheck,
		})
	}
}

func (h *HandlerBase) GenHandlerQueueStatus(queueManager *queue.QueueManager) func(c *gin.Context) {
	return func(c *gin.Context) {
		clientID := h.FetchClientID(c)
		vnumerator, vdenominator, vposition, vtotal, canAccess, userEntity := queueManager.HearbeatAndGetStatusForPage(clientID)

		if userEntity == nil {
			c.String(404, "404 page not found")
			return
		}

		c.JSON(200, gin.H{
			"canAccess":    canAccess,
			"numerator":    vnumerator,
			"denominator":  vdenominator,
			"position":     vposition,
			"total":        vtotal,
			"pathRedirect": userEntity.PathRedirect,
		})
	}
}

func (h *HandlerBase) GenHandlerQueueLeave(queueManager *queue.QueueManager) func(c *gin.Context) {
	return func(c *gin.Context) {
		clientID := h.FetchClientID(c)

		if queueManager.IsActive(clientID) {
			queueManager.ForceReleaseActiveAll()
		} else {
			queueManager.RemoveFromQueue(clientID)
		}

		c.JSON(200, gin.H{"status": "released"})
	}
}

func (h *HandlerBase) returnBusy() (bool, string) {
	if rand.Float64() < 0.5 {
		return true, "The service is currently busy. Please try again later."
	}

	return false, ""
}
