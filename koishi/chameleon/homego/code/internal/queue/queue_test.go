package queue

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
)

const (
	originalPath = "/try/"

	clientID1 = "client1"
	clientID2 = "client2"
	clientID3 = "client3"
	clientID4 = "client4"
	clientID5 = "client5"
	clientID6 = "client6"

	realMinWaitTime   = 60 * time.Second
	realMaxActiveTime = 5 * time.Minute

	minWaitTime   = 2 * time.Second
	maxActiveTime = 5 * time.Second

	heartbeatTimeout        = 1 * time.Second
	activeConnectionTimeout = 2 * time.Second
)

func sleep(t time.Duration) {
	time.Sleep(t + time.Millisecond*50)
}

func checkQueue(qm *QueueManager, clientID string) (bool, *UserEntry) {
	return qm.getQueueStatusBase(clientID, "", false)
}

func checkQueueAfter(qm *QueueManager, clientID string, t time.Duration) (bool, *UserEntry) {
	sleep(t)
	return qm.getQueueStatusBase(clientID, "", false)
}

func checkQueueHeartbeat(qm *QueueManager, clientID string, total time.Duration) {
	interval := 500 * time.Millisecond
	var elapsed time.Duration
	for elapsed < total {
		sleep(interval)
		elapsed += interval

		if elapsed > total {
			break
		}

		checkQueue(qm, clientID)
	}
}

func setActiveConnection(qm *QueueManager, clientID string) (context.Context, context.CancelFunc) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	ctx, cancel := context.WithCancel(context.Background())
	qm.DoAfterConnected(w, r, clientID, ctx, cancel)

	return ctx, cancel
}

func checkFloat(t *testing.T, name string, actual, expected float64) {
	offset := 0.00001
	if actual < expected-offset || actual > expected+offset {
		t.Error(fmt.Sprintf("%s %f should be equal to %f", name, actual, expected))
	}
}

func checkFloatRange(t *testing.T, name string, actual, expected, frange float64) {
	if actual < expected*(1-frange) || actual > expected*(1+frange) {
		t.Error(fmt.Sprintf("%s %f should be in range of %f(range %f)", name, actual, expected, frange))
	}
}

func checkVtatal(t *testing.T, name string, actual, expected int) {
	checkFloatRange(t, name, float64(actual), float64(expected), 0.1)
}

func TestNewQueueManager(t *testing.T) {
	qm := NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")

	if qm == nil {
		t.Fatal("NewQueueManager should not return nil")
	}

	if qm.queue == nil {
		t.Error("Queue should be initialized")
	}

	if len(qm.queue) != 0 {
		t.Error("Queue should be empty initially")
	}

	if qm.activeUser != nil {
		t.Error("Active user should be nil initially")
	}

	if qm.activeConnection != nil {
		t.Error("Active connection should be nil initially")
	}
}

func TestAddingQueue(t *testing.T) {
	var canAccess1 bool

	qm := NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")

	if qm.IsActive(clientID1) {
		t.Error("IsActive should return false when no active user")
	}

	user1 := qm.addToQueue(clientID1, originalPath)

	if user1.MaxWaitTime != minWaitTime {
		t.Error("user.MaxWaitTime should equal to minWaitTime")
	}

	if qm.IsActive(clientID1) {
		t.Error("IsActive should return false for now")
	}

	go checkQueueHeartbeat(qm, clientID1, minWaitTime)
	canAccess1, user1 = checkQueueAfter(qm, clientID1, minWaitTime)

	if !canAccess1 {
		t.Error("canAccess1 should return true for now")
	}

	canAccess2, user2 := checkQueue(qm, clientID2)
	if canAccess2 {
		t.Error("canAccess2 should return false for non-active client")
	}

	if user2 != nil {
		t.Error("user2 should be nil")
	}
}

func TestAddingQueueTwoUsers(t *testing.T) {
	var canAccess1 bool
	var canAccess2 bool

	qm := NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")

	user1 := qm.addToQueue(clientID1, originalPath)
	user2 := qm.addToQueue(clientID2, originalPath)

	if user1.MaxWaitTime != minWaitTime {
		t.Error("user.MaxWaitTime should equal to minWaitTime")
	}

	if user2.MaxWaitTime != maxActiveTime+minWaitTime {
		t.Error("user.MaxWaitTime should equal to maxActiveTime+minWaitTime")
	}

	go checkQueueHeartbeat(qm, clientID1, minWaitTime)
	go checkQueueHeartbeat(qm, clientID2, minWaitTime)

	sleep(minWaitTime)
	canAccess1, user1 = checkQueue(qm, clientID1)
	canAccess2, user2 = checkQueue(qm, clientID2)

	if !canAccess1 {
		t.Error("canAccess1 should return true for now")
	}

	if canAccess2 {
		t.Error("canAccess2 should return false for now")
	}
	setActiveConnection(qm, clientID1)

	sleep(maxActiveTime)
	canAccess1, user1 = checkQueue(qm, clientID1)
	canAccess2, user2 = checkQueue(qm, clientID2)

	if canAccess1 {
		t.Error("canAccess1 should return false for now")
	}
	if !canAccess2 {
		t.Error("canAccess2 should return true for now")
	}

	canAccess2, user2 = checkQueueAfter(qm, clientID2, maxActiveTime)
	if canAccess2 {
		t.Error("canAccess2 should return false for now")
	}
}

func TestHeartbeatTimeout(t *testing.T) {
	var canAccess1 bool

	qm := NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")
	user1 := qm.addToQueue(clientID1, originalPath)
	canAccess1, user1 = checkQueue(qm, clientID1)

	if user1.MaxWaitTime != minWaitTime {
		t.Error("user.MaxWaitTime should equal to minWaitTime")
	}

	if canAccess1 {
		t.Error("canAccess1 should return false for now")
	}

	if len(qm.queue) != 1 {
		t.Error("length of queue should by 1")
	}

	canAccess1, user1 = checkQueueAfter(qm, clientID1, minWaitTime)

	if canAccess1 {
		t.Error("canAccess1 should return false for now")
	}

	if len(qm.queue) != 0 {
		t.Error("length of queue should by 0")
	}
}

func TestHeartbeatTimeoutUpdateCanAccessBy(t *testing.T) {
	var canAccess1 bool
	var canAccess2 bool

	qm := NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")

	user1 := qm.addToQueue(clientID1, originalPath)
	user2 := qm.addToQueue(clientID2, originalPath)

	canAccess1, user1 = checkQueue(qm, clientID1)
	canAccess2, user2 = checkQueue(qm, clientID2)

	if user1.MaxWaitTime != minWaitTime {
		t.Error("user.MaxWaitTime should equal to minWaitTime")
	}
	if user2.MaxWaitTime != maxActiveTime+minWaitTime {
		t.Error("user.MaxWaitTime should equal to maxActiveTime+minWaitTime")
	}

	originalMaxWaitTime2A := user2.MaxWaitTime
	originalCanAccessByOffset2A := user2.CanAccessBy.Sub(time.Now())

	if canAccess1 {
		t.Error("IcanAccess1 should return false for now")
	}
	if canAccess2 {
		t.Error("IcanAccess2 should return false for now")
	}

	if len(qm.queue) != 2 {
		t.Error("length of queue should by 2")
	}

	go checkQueueHeartbeat(qm, clientID2, minWaitTime-500*time.Millisecond)

	sleep(minWaitTime - 500*time.Millisecond)
	canAccess1, user1 = checkQueue(qm, clientID1)
	canAccess2, user2 = checkQueue(qm, clientID2)

	if canAccess1 {
		t.Error("canAccess1 should return false for cleintID1")
	}

	if len(qm.queue) != 1 {
		t.Error("length of queue should by 1")
	}

	if user2.MaxWaitTime != originalMaxWaitTime2A {
		t.Error("user2.MaxWaitTime should equal to originalMaxWaitTime2")
	}

	originalCanAccessByOffset2B := user2.CanAccessBy.Sub(time.Now())
	checkFloatRange(t, "originalCanAccessByOffset2B", (originalCanAccessByOffset2A - originalCanAccessByOffset2B).Seconds(), (5 * time.Second).Seconds(), 0.1)

	go checkQueueHeartbeat(qm, clientID2, minWaitTime)
	canAccess2, user2 = checkQueueAfter(qm, clientID2, minWaitTime)

	if !canAccess2 {
		t.Error("canAccess2 should return true for cleintID2")
	}
}

func TestDisconnectUpdateCanAccessBy(t *testing.T) {
	var canAccess1 bool
	var canAccess2 bool

	qm := NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")

	user1 := qm.addToQueue(clientID1, originalPath)
	user2 := qm.addToQueue(clientID2, originalPath)

	canAccess1, user1 = checkQueue(qm, clientID1)
	canAccess2, user2 = checkQueue(qm, clientID2)

	if user1.MaxWaitTime != minWaitTime {
		t.Error("user.MaxWaitTime should equal to minWaitTime")
	}
	if user2.MaxWaitTime != maxActiveTime+minWaitTime {
		t.Error("user.MaxWaitTime should equal to maxActiveTime+minWaitTime")
	}

	if canAccess1 {
		t.Error("IcanAccess1 should return false for now")
	}
	if canAccess2 {
		t.Error("IcanAccess2 should return false for now")
	}

	if len(qm.queue) != 2 {
		t.Error("length of queue should by 2")
	}

	originalMaxWaitTime2A := user2.MaxWaitTime
	originalCanAccessByOffset2A := user2.CanAccessBy.Sub(time.Now())

	go checkQueueHeartbeat(qm, clientID1, minWaitTime)
	go checkQueueHeartbeat(qm, clientID2, minWaitTime)

	sleep(minWaitTime)
	canAccess1, user1 = checkQueue(qm, clientID1)
	canAccess2, user2 = checkQueue(qm, clientID2)

	if !canAccess1 {
		t.Error("canAccess1 should return true")
	}

	_, cancel := setActiveConnection(qm, clientID1)
	cancel()

	go checkQueueHeartbeat(qm, clientID2, activeConnectionTimeout)

	sleep(activeConnectionTimeout)

	canAccess1, user1 = checkQueue(qm, clientID1)
	canAccess2, user2 = checkQueue(qm, clientID2)

	if canAccess1 {
		t.Error("canAccess1 should return false")
	}

	if canAccess2 {
		t.Error("canAccess2 should return false")
	}

	if len(qm.queue) != 1 {
		t.Error("length of queue should by 1")
	}

	if user2.MaxWaitTime != originalMaxWaitTime2A {
		t.Error("user2.MaxWaitTime should equal to originalMaxWaitTime2")
	}

	originalCanAccessByOffset2B := user2.CanAccessBy.Sub(time.Now())
	checkFloatRange(t, "originalCanAccessByOffset2B", (originalCanAccessByOffset2A - originalCanAccessByOffset2B).Seconds(), (5 * time.Second).Seconds(), 0.1)

	go checkQueueHeartbeat(qm, clientID2, minWaitTime)
	canAccess2, user2 = checkQueueAfter(qm, clientID2, minWaitTime)

	if !canAccess2 {
		t.Error("canAccess2 should return true for cleintID2")
	}
}

func TestNoActiveConnection(t *testing.T) {
	var canAccess1 bool
	var canAccess2 bool

	qm := NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")
	qm.addToQueue(clientID1, originalPath)
	qm.addToQueue(clientID2, originalPath)

	go checkQueueHeartbeat(qm, clientID1, minWaitTime)
	go checkQueueHeartbeat(qm, clientID2, minWaitTime)

	sleep(minWaitTime)
	canAccess1, _ = checkQueue(qm, clientID1)
	canAccess2, _ = checkQueue(qm, clientID2)

	if !canAccess1 {
		t.Error("canAccess1 should return true for now")
	}

	go checkQueueHeartbeat(qm, clientID2, activeConnectionTimeout)

	canAccess1, _ = checkQueueAfter(qm, clientID1, activeConnectionTimeout)
	if canAccess1 {
		t.Error("canAccess1 should return false for now")
	}

	canAccess2, _ = checkQueue(qm, clientID2)
	if canAccess2 {
		t.Error("canAccess2 should return false for now")
	}

	go checkQueueHeartbeat(qm, clientID2, minWaitTime)
	canAccess2, _ = checkQueueAfter(qm, clientID2, minWaitTime)
	if !canAccess2 {
		t.Error("canAccess2 should return true for now")
	}
}

func TestWaitProgressTotal(t *testing.T) {
	var vtotal1, vtotal2, vtotal3, vtotal4, vtotal5, vtotal6 int
	var vnumerator2A, vnumerator2B, vdenominator2A, vdenominator2B int
	var maxWaitTime2, maxWaitTime3, maxWaitTime4 time.Duration
	const maxWaitSeconds5 = 1560 // (5*realMaxActiveTime + realMinWaitTime).Seconds()
	const maxWaitSeconds6 = 1860 // (6*realMaxActiveTime + realMinWaitTime).Seconds()

	qm := NewQueueManager(realMinWaitTime, realMaxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")

	user1 := qm.addToQueue(clientID1, originalPath)
	_, _, _, vtotal1, _, user1 = qm.HearbeatAndGetStatusForPage(clientID1)
	fmt.Println("user1.MaxWaitTime: ", user1.MaxWaitTime)
	checkVtatal(t, "vtotal1", vtotal1, maxWaitSeconds5)

	user2 := qm.addToQueue(clientID2, originalPath)
	vnumerator2A, vdenominator2A, _, vtotal2, _, user2 = qm.HearbeatAndGetStatusForPage(clientID2)
	vratio2A := float64(vnumerator2A) / float64(vdenominator2A)
	checkVtatal(t, "vtotal2", vtotal2, maxWaitSeconds5)
	maxWaitTime2 = user2.MaxWaitTime
	if maxWaitTime2 != 5*time.Minute+60*time.Second {
		t.Error("maxWaitTime2 is invalid", maxWaitTime2)
	}

	user3 := qm.addToQueue(clientID3, originalPath)
	_, _, _, vtotal3, _, user3 = qm.HearbeatAndGetStatusForPage(clientID3)
	checkVtatal(t, "vtotal3", vtotal3, maxWaitSeconds5)
	maxWaitTime3 = user3.MaxWaitTime
	if maxWaitTime3 != 2*5*time.Minute+60*time.Second {
		t.Error("maxWaitTime3 is invalid", maxWaitTime3)
	}

	user4 := qm.addToQueue(clientID4, originalPath)
	_, _, _, vtotal4, _, user4 = qm.HearbeatAndGetStatusForPage(clientID4)
	checkVtatal(t, "vtotal4", vtotal4, maxWaitSeconds5)
	maxWaitTime4 = user4.MaxWaitTime
	if maxWaitTime4 != 3*5*time.Minute+60*time.Second {
		t.Error("maxWaitTime4 is invalid", maxWaitTime4)
	}

	user5 := qm.addToQueue(clientID5, originalPath)
	_, _, _, vtotal5, _, user5 = qm.HearbeatAndGetStatusForPage(clientID5)
	checkVtatal(t, "vtotal5", vtotal5, maxWaitSeconds5)
	fmt.Println("user5.MaxWaitTime: ", user5.MaxWaitTime)

	user6 := qm.addToQueue(clientID6, originalPath)
	_, _, _, vtotal6, _, user6 = qm.HearbeatAndGetStatusForPage(clientID6)
	checkVtatal(t, "vtotal6", vtotal6, maxWaitSeconds6)
	fmt.Println("user6.MaxWaitTime: ", user6.MaxWaitTime)

	go checkQueueHeartbeat(qm, clientID2, heartbeatTimeout)
	go checkQueueHeartbeat(qm, clientID3, heartbeatTimeout)
	go checkQueueHeartbeat(qm, clientID4, heartbeatTimeout)
	go checkQueueHeartbeat(qm, clientID5, heartbeatTimeout)
	go checkQueueHeartbeat(qm, clientID6, heartbeatTimeout)
	sleep(heartbeatTimeout)

	_, _, _, vtotal1, _, user1 = qm.HearbeatAndGetStatusForPage(clientID1)
	if user1 != nil {
		t.Error("user1 should be nil")
	}

	vnumerator2B, vdenominator2B, _, vtotal2, _, user2 = qm.HearbeatAndGetStatusForPage(clientID2)
	fmt.Println("user2.MaxWaitTime: ", user2.MaxWaitTime)
	vratio2B := float64(vnumerator2B) / float64(vdenominator2B)
	checkFloatRange(t, "vratio2 in range", vratio2A-vratio2B, 0.8, 0.1)
	if user2.MaxWaitTime != maxWaitTime2 {
		t.Error("user2.MaxWaitTime should be maxWaitTime2")
	}

	_, _, _, vtotal3, _, user3 = qm.HearbeatAndGetStatusForPage(clientID3)
	if user3.MaxWaitTime != maxWaitTime3 {
		t.Error("user3.MaxWaitTime should be maxWaitTime3")
	}

	_, _, _, vtotal4, _, user4 = qm.HearbeatAndGetStatusForPage(clientID4)
	if user4.MaxWaitTime != maxWaitTime4 {
		t.Error("user4.MaxWaitTime should be maxWaitTime4")
	}
}

func TestMaxActiveTime(t *testing.T) {
	var canAccess1 bool

	qm := NewQueueManager(minWaitTime, maxActiveTime, heartbeatTimeout, activeConnectionTimeout, "")
	qm.addToQueue(clientID1, originalPath)

	go checkQueueHeartbeat(qm, clientID1, minWaitTime)
	canAccess1, _ = checkQueueAfter(qm, clientID1, minWaitTime)
	if !canAccess1 {
		t.Error("canAccess1 should return true for now")
	}

	ctx, _ := setActiveConnection(qm, clientID1)
	canAccess1, _ = checkQueueAfter(qm, clientID1, maxActiveTime)

	if canAccess1 {
		t.Error("canAccess1 should return false")
	}

	if ctx.Err() != nil {
		t.Log("Context disconnected successfully")
	} else {
		t.Error("Context should be disconnected")
	}
}
