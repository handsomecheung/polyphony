package queue

import (
	"context"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"koishi/chameleon/homego/internal/common"
	"koishi/chameleon/homego/internal/k8s"
)

type UserEntry struct {
	ClientID        string
	Position        int
	PathRedirect    string
	LastHeartbeatAt time.Time
	LastConnectedAt time.Time
	MaxWaitTime     time.Duration
	CanAccessBy     time.Time
	ExpirationAt    time.Time
	PromotionAt     time.Time
}

type ActiveConnection struct {
	User       *UserEntry
	Response   http.ResponseWriter
	Request    *http.Request
	StartAt    time.Time
	CancelFunc context.CancelFunc
}

type QueueManager struct {
	mutex                   sync.RWMutex
	queue                   []UserEntry
	activeUser              *UserEntry
	activeAt                time.Time
	activeConnection        *ActiveConnection
	activeTimer             *time.Timer
	MinWaitTime             time.Duration
	MaxActiveTime           time.Duration
	HeartbeatTimeout        time.Duration
	ActiveConnectionTimeout time.Duration
	VirtualTotal            int
	VirtualTotalUpdated     time.Time
	deploymentToRestart     string
}

func NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout time.Duration, deploymentToRestart string) *QueueManager {
	qm := &QueueManager{
		queue:                   make([]UserEntry, 0),
		MinWaitTime:             minWaitTime,
		MaxActiveTime:           maxActiveTime,
		HeartbeatTimeout:        heartbeatTimeout,
		ActiveConnectionTimeout: activeConnectionTimeout,
		deploymentToRestart:     deploymentToRestart,
	}

	qm.updateVirtaulTotal(true)

	return qm
}

func (qm *QueueManager) findInQueue(clientID string) int {
	for i, entry := range qm.queue {
		if entry.ClientID == clientID {
			return i
		}
	}
	return -1
}

func (qm *QueueManager) updateVirtaulTotal(force bool) {
	if force || time.Since(qm.VirtualTotalUpdated) > 60*time.Second {
		vmaxLength := 5
		vlength := max(len(qm.queue), vmaxLength)

		baseRatio := 5 * time.Minute.Seconds()
		vtotal := common.RandIntRange(int(qm.calMaxWaitTime(vlength).Seconds()), 0.1) / int(math.Ceil(qm.MaxActiveTime.Seconds()/baseRatio))

		qm.VirtualTotal = vtotal
		qm.VirtualTotalUpdated = time.Now()
	}
}

func (qm *QueueManager) IsActive(clientID string) bool {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	if qm.activeUser == nil {
		return false
	}

	return qm.activeUser.ClientID == clientID
}

func (qm *QueueManager) releaseActiveAll() {
	if qm.activeUser == nil {
		return
	}

	qm.closeActiveConnection()
	qm.stopActiveTimer()

	qm.activeUser = nil
	qm.activeAt = time.Time{}
	qm.activeConnection = nil

	if qm.deploymentToRestart != "" {
		k8s.RestartDeployment(qm.deploymentToRestart)
	}

	qm.tryPromoteNext()
}

func (qm *QueueManager) ForceReleaseActiveAll() {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	qm.releaseActiveAll()
}

func (qm *QueueManager) ForceReleaseActiveConnection() {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	qm.activeConnection = nil
	if qm.activeUser != nil {
		qm.activeUser.LastConnectedAt = time.Now()
	}
}

func (qm *QueueManager) RemoveFromQueue(clientID string) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	if idx := qm.findInQueue(clientID); idx >= 0 {
		qm.removeFromQueue(idx)
	}
}

func (qm *QueueManager) removeFromQueue(idx int) {
	if idx < 0 || idx >= len(qm.queue) {
		return
	}

	// Remove from queue
	copy(qm.queue[idx:], qm.queue[idx+1:])
	qm.queue = qm.queue[:len(qm.queue)-1]

	// Update positions
	for i := idx; i < len(qm.queue); i++ {
		qm.queue[i].Position = i
	}
}

func (qm *QueueManager) cleanupUsersWithoutHeartbeat() bool {
	now := time.Now()
	var indicesToRemove []int

	for i, entry := range qm.queue {
		if now.Sub(entry.LastHeartbeatAt) > qm.HeartbeatTimeout {
			log.Printf("Removing client %s from queue due to heartbeat timeout", entry.ClientID)
			indicesToRemove = append(indicesToRemove, i)
		}
	}

	for i := len(indicesToRemove) - 1; i >= 0; i-- {
		qm.removeFromQueue(indicesToRemove[i])
	}

	return len(indicesToRemove) > 0
}

func (qm *QueueManager) cleanupActiveUserNoConnection() bool {
	if qm.activeUser == nil || qm.activeConnection != nil {
		return false
	}

	// Check if active user exists but no connection for more than 1 minute
	// Only check timeout if user has connected before (lastConnectedTime is not zero)
	if !qm.activeUser.LastConnectedAt.IsZero() && time.Since(qm.activeUser.LastConnectedAt) >= qm.ActiveConnectionTimeout {
		log.Printf("Active user %s has no connection for some time, releasing", qm.activeUser.ClientID)
		qm.releaseActiveAll()

		return true
	}

	if qm.activeUser.LastConnectedAt.IsZero() && time.Since(qm.activeUser.PromotionAt) >= qm.ActiveConnectionTimeout {
		log.Printf("Active user %s has never connection from promotion, releasing", qm.activeUser.ClientID)
		qm.releaseActiveAll()

		return true
	}

	return false
}

func (qm *QueueManager) tryPromoteNext() {
	if len(qm.queue) == 0 {
		return
	}

	if qm.activeUser != nil {
		return
	}

	q := qm.queue[0]
	now := time.Now()
	if now.After(q.CanAccessBy) {
		log.Printf("Promoting client: %s", q.ClientID)

		qCopy := q
		qCopy.ExpirationAt = now.Add(qm.MaxActiveTime)
		qCopy.PromotionAt = now
		qm.activeUser = &qCopy
		qm.activeAt = now
		qm.startActiveTimer()

		qm.removeFromQueue(0)
		return
	}
}

func (qm *QueueManager) updateCanAccessBy() {
	now := time.Now()
	for i, _ := range qm.queue {
		maxWaitTime := qm.calMaxWaitTime(i)
		qm.queue[i].CanAccessBy = now.Add(maxWaitTime)
	}
}

func (qm *QueueManager) calMaxWaitTime(usersBefore int) time.Duration {
	return time.Duration(usersBefore)*qm.MaxActiveTime + qm.MinWaitTime
}

func (qm *QueueManager) getQueueStatusBase(clientID, originalPath string, doAdd bool) (bool, *UserEntry) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	if qm.activeUser != nil && time.Now().After(qm.activeUser.ExpirationAt) {
		log.Printf("client %s has expirated", clientID)
		qm.releaseActiveAll()
	}

	if qm.cleanupActiveUserNoConnection() || qm.cleanupUsersWithoutHeartbeat() {
		log.Println("update all CanAccessBy after cleanup users")
		qm.updateCanAccessBy()
	}

	qm.tryPromoteNext()

	// Check if already active (after potential promotion)
	if qm.activeUser != nil && qm.activeUser.ClientID == clientID {
		return true, qm.activeUser
	}

	now := time.Now()
	if idx := qm.findInQueue(clientID); idx >= 0 {
		currentUser := &qm.queue[idx]
		currentUser.LastHeartbeatAt = now
		return false, currentUser
	}

	if doAdd {
		return false, qm.addToQueue(clientID, originalPath)
	} else {
		return false, nil
	}
}

func (qm *QueueManager) addToQueue(clientID, originalPath string) *UserEntry {
	now := time.Now()
	maxWaitTime := qm.calMaxWaitTime(len(qm.queue))
	user := UserEntry{
		ClientID:        clientID,
		Position:        len(qm.queue),
		LastHeartbeatAt: now,
		PathRedirect:    originalPath,
		MaxWaitTime:     maxWaitTime,
		CanAccessBy:     now.Add(maxWaitTime),
		PromotionAt:     now,
	}
	qm.queue = append(qm.queue, user)
	qm.updateVirtaulTotal(true)

	return &user
}

func (qm *QueueManager) HearbeatAndGetStatusForPage(clientID string) (int, int, int, int, bool, *UserEntry) {
	canAccess, userEntity := qm.getQueueStatusBase(clientID, "", false)

	if userEntity == nil {
		return 0, 0, 0, 0, canAccess, userEntity
	}

	qm.updateVirtaulTotal(false)
	vnumerator := max(int(time.Since(userEntity.CanAccessBy).Seconds())*-1, 0)
	vdenominator := int(userEntity.MaxWaitTime.Seconds())
	vposition := vnumerator

	return vnumerator, vdenominator, vposition, qm.VirtualTotal, canAccess, userEntity
}

func (qm *QueueManager) CanAccess(clientID string, originalPath string) bool {
	canAccess, _ := qm.getQueueStatusBase(clientID, originalPath, true)
	return canAccess
}

func (qm *QueueManager) GetQueueLength() int {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	return len(qm.queue)
}

func (qm *QueueManager) SetActiveConnection(clientID string, w http.ResponseWriter, r *http.Request, cancelFunc context.CancelFunc) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	if qm.activeUser != nil && qm.activeUser.ClientID == clientID {
		log.Println("set active connection for client: ", clientID)
		now := time.Now()
		qm.activeConnection = &ActiveConnection{
			User:       qm.activeUser,
			Response:   w,
			Request:    r,
			StartAt:    now,
			CancelFunc: cancelFunc,
		}
		qm.activeUser.LastConnectedAt = now
	}
}

func (qm *QueueManager) closeActiveConnection() {
	if qm.activeConnection == nil {
		return
	}

	log.Printf("Canceling active connection for client: %s", qm.activeConnection.User.ClientID)

	if qm.activeConnection.CancelFunc != nil {
		qm.activeConnection.CancelFunc()
	}
}

func (qm *QueueManager) ForceCloseActiveConnection() {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	qm.closeActiveConnection()
}

func (qm *QueueManager) startActiveTimer() {
	if qm.activeTimer != nil {
		qm.activeTimer.Stop()
	}

	qm.activeTimer = time.AfterFunc(qm.MaxActiveTime, func() {
		log.Printf("Active user timeout - force releasing active connection and position")
		qm.ForceCloseActiveConnection()
		qm.ForceReleaseActiveAll()
	})
}

func (qm *QueueManager) stopActiveTimer() {
	if qm.activeTimer != nil {
		qm.activeTimer.Stop()
		qm.activeTimer = nil
	}
}

type AdminStatus struct {
	ActiveUser       *UserEntry      `json:"activeUser"`
	ActiveTime       time.Time       `json:"activeTime"`
	Queue            []UserEntry     `json:"queue"`
	QueueLength      int             `json:"queueLength"`
	ActiveConnection *ConnectionInfo `json:"activeConnection"`
}

type ConnectionInfo struct {
	ClientID       string    `json:"clientID"`
	StartTime      time.Time `json:"startTime"`
	Duration       string    `json:"duration"`
	RequestURL     string    `json:"requestURL"`
	UserAgent      string    `json:"userAgent"`
	RemoteAddr     string    `json:"remoteAddr"`
	CFIpcountry    string    `json:"cfIpcountry"`
	CFConnectingIp string    `json:"cfConnectingIp"`
}

func (qm *QueueManager) GetAdminStatus() AdminStatus {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	status := AdminStatus{
		ActiveUser:  qm.activeUser,
		ActiveTime:  qm.activeAt,
		Queue:       make([]UserEntry, len(qm.queue)),
		QueueLength: len(qm.queue),
	}

	// Copy queue to avoid race conditions
	copy(status.Queue, qm.queue)

	if qm.activeConnection != nil {
		status.ActiveConnection = &ConnectionInfo{
			ClientID:       qm.activeConnection.User.ClientID,
			StartTime:      qm.activeConnection.StartAt,
			Duration:       time.Since(qm.activeConnection.StartAt).String(),
			RequestURL:     qm.activeConnection.Request.URL.String(),
			UserAgent:      qm.activeConnection.Request.UserAgent(),
			RemoteAddr:     qm.activeConnection.Request.RemoteAddr,
			CFIpcountry:    qm.activeConnection.Request.Header.Get("CF-IPCountry"),
			CFConnectingIp: qm.activeConnection.Request.Header.Get("CF-Connecting-IP"),
		}
	}

	return status
}

func (qm *QueueManager) DoAfterConnected(writer http.ResponseWriter, request *http.Request, clientID string, ctx context.Context, cancel context.CancelFunc) {
	qm.SetActiveConnection(clientID, writer, request, cancel)

	// Monitor connection close event
	go func() {
		<-ctx.Done() // Wait for context cancellation (connection close)
		log.Printf("WebSocket connection closed for client: %s. unset active connection", clientID)
		qm.ForceReleaseActiveConnection()
	}()
}
