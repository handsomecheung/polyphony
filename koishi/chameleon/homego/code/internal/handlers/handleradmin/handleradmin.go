package handleradmin

import (
	"fmt"
	"os"
	"path"

	"koishi/chameleon/homego/internal/handlers/handlerbase"
	"koishi/chameleon/homego/internal/handlers/handlerlinux"
	"koishi/chameleon/homego/internal/handlers/handlerps1"
	"koishi/chameleon/homego/internal/queue"

	"github.com/gin-gonic/gin"
)

const (
	PathPrefix = "/admin"
)

var (
	pathQueueStatusPage = path.Join(PathPrefix, "/queues")

	hostPublic  string
	hostPrivate string
)

func init() {
	hostPublic = os.Getenv("HOST_PUBLIC")
	if hostPublic == "" {
		panic("HOST_PUBLIC environment variable is not set")
	}

	hostPrivate = os.Getenv("HOST_PRIVATE")
	if hostPrivate == "" {
		panic("HOST_PRIVATE environment variable is not set")
	}
}

type HandlerAdmin struct {
	*handlerbase.HandlerBase
	QueueManagerPS1GOTTY   *queue.QueueManager
	QueueManagerPS1MBTTY   *queue.QueueManager
	QueueManagerLinuxNOVNC *queue.QueueManager
	QueueManagerLinuxMBVNC *queue.QueueManager
}

func NewHandler(base *handlerbase.HandlerBase, ps1 *handlerps1.HandlerPS1, linux *handlerlinux.HandlerLinux) *HandlerAdmin {
	return &HandlerAdmin{
		HandlerBase:            base,
		QueueManagerPS1GOTTY:   ps1.QueueManagerGOTTY,
		QueueManagerPS1MBTTY:   ps1.QueueManagerMBTTY,
		QueueManagerLinuxNOVNC: linux.QueueManagerNOVNC,
		QueueManagerLinuxMBVNC: linux.QueueManagerMBVNC,
	}
}

type Link struct {
	URL         string
	Name        string
	DisplayName string
}

func (h *HandlerAdmin) DeleteSessions(c *gin.Context) {
	message := h.AuthService.LogoutAll()
	c.JSON(200, gin.H{"message": message})
}

func (h *HandlerAdmin) LinksPage(c *gin.Context) {
	h.TrySetFakeSessionID(c)

	links := []Link{
		{URL: fmt.Sprintf("%s%s", hostPrivate, pathQueueStatusPage), Name: "queue status", DisplayName: "⚡ Admin Queue Status"},
		{URL: fmt.Sprintf("%s%s", hostPublic, handlerps1.PathArticels), Name: "ps1 articles", DisplayName: "💻 PowerShell Articles"},
		{URL: fmt.Sprintf("%s%s", hostPublic, handlerps1.PathTry), Name: "ps1 tty", DisplayName: "💻 PowerShell TTY"},
		{URL: fmt.Sprintf("%s%s", hostPublic, handlerlinux.PathArticels), Name: "linux articles", DisplayName: "🐧 Linux Articles"},
		{URL: fmt.Sprintf("%s%s", hostPublic, handlerlinux.PathTry), Name: "linux gui", DisplayName: "🐧 Linux GUI"},
	}

	c.HTML(200, "admin-links.html", gin.H{
		"title": "Admin Links",
		"links": links,
	})
}

type QueueStatusData struct {
	Name        string
	DisplayName string
	Icon        string
	Status      interface{}
}

func (h *HandlerAdmin) QueuePage(c *gin.Context) {
	gottyStatus := h.QueueManagerPS1GOTTY.GetAdminStatus()
	mbttyStatus := h.QueueManagerPS1MBTTY.GetAdminStatus()
	novncStatus := h.QueueManagerLinuxNOVNC.GetAdminStatus()
	mbvncStatus := h.QueueManagerLinuxMBVNC.GetAdminStatus()

	queueStatuses := []QueueStatusData{
		{Name: "mbtty", DisplayName: "PS1 MBTTY Queue (Authenticated)", Icon: "🔐", Status: mbttyStatus},
		{Name: "mbvnc", DisplayName: "Linux MBVNC Queue (Authenticated)", Icon: "🔒", Status: mbvncStatus},
		{Name: "gotty", DisplayName: "PS1 GOTTY Queue", Icon: "💻", Status: gottyStatus},
		{Name: "novnc", DisplayName: "Linux NOVNC Queue", Icon: "🐧", Status: novncStatus},
	}

	c.HTML(200, "admin-queue.html", gin.H{
		"queueStatuses": queueStatuses,
	})
}
