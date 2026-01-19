package handlerps1

import (
	"path"
	"time"

	"koishi/chameleon/homego/internal/handlers/handlerbase"
	"koishi/chameleon/homego/internal/queue"

	"github.com/gin-gonic/gin"
)

const (
	category   = "PowerShell"
	PathPrefix = "/ps1"

	gottyMinWaitTime   = 60 * time.Second
	gottyMaxActiveTime = 5 * time.Minute

	mbttyMinWaitTime   = 10 * time.Second
	mbttyMaxActiveTime = 5 * time.Hour

	heartbeatTimeout        = 30 * time.Second
	activeConnectionTimeout = 60 * time.Second

	deploymentGOTTY = "gotty"
)

var (
	PathArticels   = path.Join(PathPrefix, "/")
	PathTry        = path.Join(PathPrefix, "/try/")
	PathRanks      = path.Join(PathPrefix, "/article-ranks")
	pathQueuePage  = path.Join(PathPrefix, "/queue")
	pathQueueLeave = path.Join(PathPrefix, "/queue")
	pathQueueCheck = path.Join(PathPrefix, "/queue/status")
)

type HandlerPS1 struct {
	*handlerbase.HandlerBase
	QueueManagerGOTTY *queue.QueueManager
	QueueManagerMBTTY *queue.QueueManager
}

func NewHandler(base *handlerbase.HandlerBase) *HandlerPS1 {
	return &HandlerPS1{
		HandlerBase:       base,
		QueueManagerGOTTY: queue.NewQueueManager(gottyMinWaitTime, gottyMaxActiveTime, heartbeatTimeout, activeConnectionTimeout, deploymentGOTTY),
		QueueManagerMBTTY: queue.NewQueueManager(mbttyMinWaitTime, mbttyMaxActiveTime, heartbeatTimeout, activeConnectionTimeout, ""),
	}
}

func (h *HandlerPS1) IndexPage(c *gin.Context) {
	h.GenHandlerIndexPage(category, PathPrefix)(c)
}

func (h *HandlerPS1) RanksPage(c *gin.Context) {
	h.GenHandlerArticleRanksPage(category, PathPrefix)(c)
}

func (h *HandlerPS1) ShowArticlePage(c *gin.Context) {
	h.GenHandlerShowArticlePage(PathArticels, PathTry, PathRanks)(c)
}

func (h *HandlerPS1) getTryHost(c *gin.Context) string {
	targetHost := "http://gotty"
	if h.CheckAuth(c) {
		targetHost = "http://mbtty"
	}
	return targetHost
}

func (h *HandlerPS1) Try(c *gin.Context) {
	h.TryBase(c, true, h.getQueueManager(c), h.getTryHost(c), PathTry, pathQueuePage, "webtty")
}

func (h *HandlerPS1) getQueueManager(c *gin.Context) *queue.QueueManager {
	queueManager := h.QueueManagerGOTTY
	if h.CheckAuth(c) {
		queueManager = h.QueueManagerMBTTY
	}
	return queueManager
}

func (h *HandlerPS1) QueuePage(c *gin.Context) {
	h.GenHandlerQueuePage(h.getQueueManager(c), PathArticels, pathQueueLeave, pathQueueCheck)(c)
}

func (h *HandlerPS1) QueueStatus(c *gin.Context) {
	h.GenHandlerQueueStatus(h.getQueueManager(c))(c)
}

func (h *HandlerPS1) QueueLeave(c *gin.Context) {
	h.GenHandlerQueueLeave(h.getQueueManager(c))(c)
}
