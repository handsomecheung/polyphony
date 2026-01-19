package handlerlinux

import (
	"path"
	"time"

	"koishi/chameleon/homego/internal/handlers/handlerbase"
	"koishi/chameleon/homego/internal/queue"

	"github.com/gin-gonic/gin"
)

const (
	category   = "Linux"
	PathPrefix = "/linux"

	novncMinWaitTime   = 5 * time.Minute
	novncMaxActiveTime = 10 * time.Minute

	mbvncMinWaitTime   = 10 * time.Second
	mbvncMaxActiveTime = 5 * time.Hour

	heartbeatTimeout        = 30 * time.Second
	activeConnectionTimeout = 60 * time.Second

	deploymentNOVNC = "novnc"
)

var (
	PathArticels   = path.Join(PathPrefix, "/")
	PathTry        = path.Join(PathPrefix, "/try/?autoconnect=true")
	PathRanks      = path.Join(PathPrefix, "/article-ranks")
	pathQueuePage  = path.Join(PathPrefix, "/queue")
	pathQueueLeave = path.Join(PathPrefix, "/queue")
	pathQueueCheck = path.Join(PathPrefix, "/queue/status")
)

type HandlerLinux struct {
	*handlerbase.HandlerBase
	QueueManagerNOVNC *queue.QueueManager
	QueueManagerMBVNC *queue.QueueManager
}

func NewHandler(base *handlerbase.HandlerBase) *HandlerLinux {
	return &HandlerLinux{
		HandlerBase:       base,
		QueueManagerNOVNC: queue.NewQueueManager(novncMinWaitTime, novncMaxActiveTime, heartbeatTimeout, activeConnectionTimeout, deploymentNOVNC),
		QueueManagerMBVNC: queue.NewQueueManager(mbvncMinWaitTime, mbvncMaxActiveTime, heartbeatTimeout, activeConnectionTimeout, ""),
	}
}

func (h *HandlerLinux) IndexPage(c *gin.Context) {
	h.GenHandlerIndexPage(category, PathPrefix)(c)
}

func (h *HandlerLinux) RanksPage(c *gin.Context) {
	h.GenHandlerArticleRanksPage(category, PathPrefix)(c)
}

func (h *HandlerLinux) ShowArticlePage(c *gin.Context) {
	h.GenHandlerShowArticlePage(PathArticels, PathTry, PathRanks)(c)
}

func (h *HandlerLinux) getTryHost(c *gin.Context) string {
	targetHost := "http://novnc/"
	if h.CheckAuth(c) {
		targetHost = "http://mbvnc/"
	}
	return targetHost
}

func (h *HandlerLinux) getQueueManager(c *gin.Context) *queue.QueueManager {
	queueManager := h.QueueManagerNOVNC
	if h.CheckAuth(c) {
		queueManager = h.QueueManagerMBVNC
	}
	return queueManager
}

func (h *HandlerLinux) Try(c *gin.Context) {
	name := c.Param("name")
	if name == "" || name == "/" {
		c.Request.URL.Path = path.Join(c.Request.URL.Path, "vnc.html")
	}
	h.TryBase(c, true, h.getQueueManager(c), h.getTryHost(c), PathTry, pathQueuePage, "")
}

func (h *HandlerLinux) QueuePage(c *gin.Context) {
	h.GenHandlerQueuePage(h.getQueueManager(c), PathArticels, pathQueueLeave, pathQueueCheck)(c)
}

func (h *HandlerLinux) QueueStatus(c *gin.Context) {
	h.GenHandlerQueueStatus(h.getQueueManager(c))(c)
}

func (h *HandlerLinux) QueueLeave(c *gin.Context) {
	h.GenHandlerQueueLeave(h.getQueueManager(c))(c)
}
