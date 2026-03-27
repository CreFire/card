package netmgr

import (
	"fmt"
	"game/deps/basal"
	"game/deps/fastid"
	"game/deps/msg"
	"game/deps/netmgr/options"
	timermgr "game/deps/timer_mgr"
	"game/deps/xlog"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/samber/lo"
)

const (
	loginTimeout             = time.Minute
	loginCheckInterval       = 10 * time.Second
	stopFrontTimeout         = 5 * time.Second
	internalStopWriteTimeout = 5 * time.Second
	externalStopWriteTimeout = 50 * time.Millisecond
)

type NetMgr struct {
	listenMap     map[string]IMsgQue           // as server
	sessMap       map[int64]IMsgQue            // include as client & server
	sessAgent     map[string]map[int32]IMsgQue // svrName : svrId : conn
	taskChan      chan func()
	stopCh        chan struct{}
	startOnce     sync.Once
	stopOnce      sync.Once
	wg            sync.WaitGroup
	timerMgr      *timermgr.TimerMgr
	canReconnect  func(params *options.ConnectParams) bool
	optMu         sync.RWMutex
	opt           *options.NetOptions
	localSvrType  string
	localSvrId    int32
	stopping      atomic.Bool
	acceptSessNum atomic.Int64
	loginPending  map[int64]struct{}
}

func (mgr *NetMgr) getOpt() *options.NetOptions {
	mgr.optMu.RLock()
	opt := mgr.opt
	mgr.optMu.RUnlock()
	return opt
}

func (mgr *NetMgr) setOpt(opt *options.NetOptions) {
	mgr.optMu.Lock()
	mgr.opt = opt
	mgr.optMu.Unlock()
}

func (mgr *NetMgr) SetOptions(opt *options.NetOptions) {
	mgr.addTask(func() {
		cur := mgr.getOpt()
		newOpt := options.MergeOptions(cur, opt)
		mgr.setOpt(newOpt)
	})
}

func (mgr *NetMgr) RegisterTimerMgr(timer *timermgr.TimerMgr) {
	mgr.timerMgr = timer
}

func (mgr *NetMgr) SetLocalServer(svrType string, svrId int32) {
	mgr.localSvrType = svrType
	mgr.localSvrId = svrId
}

func (mgr *NetMgr) Start() {
	mgr.startOnce.Do(func() {
		xlog.Infof("start to run net mgr ...")
		mgr.wg.Add(1)
		ticker := time.NewTicker(loginCheckInterval)
		basal.SafeGo(func() {
			defer mgr.wg.Done()
			defer ticker.Stop()
			mgr.run(ticker)
		})
	})
}

func (mgr *NetMgr) run(ticker *time.Ticker) {
	for {
		select {
		case <-mgr.stopCh:
			mgr.drainTasks()
			xlog.Infof("stop all session ...")
			mgr.stopAllSess()
			return
		case task := <-mgr.taskChan:
			mgr.safeRun(task)
		case now := <-ticker.C:
			mgr.handleLoginTimeout(now)
		}
	}
}

func (mgr *NetMgr) addTask(f func()) bool {
	if f == nil {
		return false
	}
	if mgr.stopping.Load() {
		return false
	}

	select {
	case mgr.taskChan <- f:
	default:
		xlog.Errorf("net task chan full.")
		return false
	}

	return true
}

func (mgr *NetMgr) isStopping() bool {
	return mgr.stopping.Load()
}

func (mgr *NetMgr) runTaskSync(f func()) bool {
	if f == nil {
		return false
	}

	done := make(chan struct{})
	if mgr.addTask(func() {
		defer close(done)
		f()
	}) {
		<-done
		return true
	}

	if mgr.isStopping() {
		mgr.safeRun(f)
		return true
	}

	return false
}

func (mgr *NetMgr) safeRun(f func()) {
	if f == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			xlog.Errorf("%v : %s", r, debug.Stack())
		}
	}()

	f()
}

func (mgr *NetMgr) drainTasks() {
	for {
		select {
		case task := <-mgr.taskChan:
			mgr.safeRun(task)
		default:
			return
		}
	}
}

func (mgr *NetMgr) RegisterCanReconnect(f func(params *options.ConnectParams) bool) {
	mgr.canReconnect = f
}

func (mgr *NetMgr) CanReconnect(params *options.ConnectParams) bool {
	if mgr.canReconnect != nil {
		return mgr.canReconnect(params)
	}
	return false
}

func (mgr *NetMgr) RegisterSess(svrType string, svrId int32, sessId int64) {
	mgr.addTask(func() {
		mq := mgr.sessMap[sessId]
		if mq == nil {
			return
		}
		xlog.Infof("[%s][svrId=%d] connected with sessid = %d.", svrType, svrId, sessId)

		mgr.incSessAgent(svrType, svrId, mq)
		mgr.clearLoginPending(sessId)

		mq.GetAgent().AddSvrAgt(svrType, svrId)
	})
}

func (mgr *NetMgr) KickSession(sessId int64, gid int64) {
	mgr.addTask(func() {
		mq, _ := mgr.sessMap[sessId]
		if mq == nil || (gid != 0 && gid != mq.GetAgent().GetCltUser()) {
			return
		}
		xlog.Infof("kick client with [sessid=%d] & [userid=%v].", sessId, gid)
		mgr.deleteSess(sessId)

		svrType, _ := mq.GetAgent().GetSvrAgt()
		if svrType != "" {
			mgr.decSessAgent(svrType, sessId)
		}

		// stop() may block waiting for IO; do it outside NetMgr task loop.
		basal.SafeGo(func() { mq.stop() })
	})
}

func (mgr *NetMgr) RemoveSession(sessId int64) {
	mgr.addTask(func() {
		mq, _ := mgr.sessMap[sessId]
		if mq == nil {
			return
		}
		mgr.deleteSess(sessId)

		xlog.Infof("[sessid=%v] conn removed.", sessId)

		svrType, _ := mq.GetAgent().GetSvrAgt()
		if svrType != "" {
			mgr.decSessAgent(svrType, sessId)
		}

		// stop() may block waiting for IO; do it outside NetMgr task loop.
		basal.SafeGo(func() { mq.stop() })
	})
}

func (mgr *NetMgr) RemoveSvr(svrType string, svrId int32) {
	mgr.addTask(func() {
		mq := mgr.sessAgent[svrType][svrId]
		if mq == nil {
			return
		}
		mgr.deleteSess(mq.SessId())
		xlog.Infof("[svrType:%v | svrId:%v] conn removed.", svrType, svrId)

		mgr.decSessAgent(svrType, mq.SessId())

		// stop() may block waiting for IO; do it outside NetMgr task loop.
		basal.SafeGo(func() { mq.stop() })
	})
}

func (mgr *NetMgr) Stop() {
	xlog.Infof("stop net mgr ...")
	mgr.stopOnce.Do(func() {
		mgr.stopping.Store(true)
		close(mgr.stopCh)
	})

	mgr.wg.Wait()
	xlog.Infof("net mgr stopped.")
}

func (mgr *NetMgr) StopFront() {
	xlog.Infof("stop net listen ...")
	mgr.stopListen()

	xlog.Infof("stop listen sess ...")

	if mgr.isStopping() {
		return
	}

	var stopQus []IMsgQue
	mgr.runTaskSync(func() {
		stopQus = make([]IMsgQue, 0, len(mgr.sessMap))
		for sid, mq := range mgr.sessMap {
			if mq.GetConnectType() == ConnTypeConn {
				continue
			}

			svrType, svrId := mq.GetAgent().GetSvrAgt()
			if svrType != "" {
				delete(mgr.sessAgent[svrType], svrId)
			}

			mgr.deleteSess(sid)
			stopQus = append(stopQus, mq)
		}
	})

	mgr.stopSync(stopQus)

	xlog.Infof("net front stopped.")
}

func (mgr *NetMgr) stopSync(mqs []IMsgQue) {
	if len(mqs) == 0 {
		return
	}
	var wg sync.WaitGroup
	for _, mq := range mqs {
		if mq == nil {
			continue
		}
		mq := mq
		wg.Add(1)
		basal.SafeGo(func() {
			defer wg.Done()
			if !stopWithTimeout(mq, stopFrontTimeout) {
				xlog.Warnf("stop sess timeout [sessid=%d addr=%s] %s",
					mq.SessId(), mq.remoteAddr(), mq.GetAgent().String())
			}
		})
	}
	wg.Wait()
}

func stopWithTimeout(mq IMsgQue, timeout time.Duration) bool {
	if mq == nil {
		return true
	}
	if timeout <= 0 {
		mq.stop()
		return true
	}
	done := make(chan struct{})
	basal.SafeGo(func() {
		defer close(done)
		mq.stop()
	})
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (mgr *NetMgr) stopListen() {
	for _, v := range mgr.listenMap {
		v.stop()
	}
}

func (mgr *NetMgr) stopAllSess() {
	for _, v := range mgr.sessMap {
		v.stop()
	}
}

func (mgr *NetMgr) incSessAgent(svrType string, svrId int32, mq IMsgQue) {
	if mgr.sessAgent[svrType] == nil {
		mgr.sessAgent[svrType] = make(map[int32]IMsgQue)
	}
	mgr.sessAgent[svrType][svrId] = mq
}

func (mgr *NetMgr) GetSessNum() int {
	return len(mgr.sessMap)
}

func (mgr *NetMgr) GetAcceptSessNum() int {
	return int(mgr.acceptSessNum.Load())
}

func (mgr *NetMgr) incAcceptSessNum() {
	mgr.acceptSessNum.Add(1)
}

func (mgr *NetMgr) decAcceptSessNum() {
	newCount := mgr.acceptSessNum.Add(-1)
	if newCount < 0 {
		xlog.Errorf("accept sess num is negative.")
		mgr.acceptSessNum.Add(1)
	}
}

func (mgr *NetMgr) StartListen(opt *options.NetOptions, handler INetEventHandler) error {
	if opt == nil || opt.ListenParams == nil {
		return fmt.Errorf("msgque options is nil")
	}
	if err := opt.CheckOptions(); err != nil {
		return err
	}
	mgr.setOpt(opt)

	addr := opt.ListenParams.ListenAddr
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("[%s] start listen failed : %v", addr, err)
	}
	xlog.Infof("[tcp] start listen [%s]", addr)
	mqListen := newTcpListen(ln, opt, handler)
	mgr.listenMap[addr] = mqListen
	mqListen.listen(mgr)
	return nil
}

func (mgr *NetMgr) StartConnect(opt *options.NetOptions, handler INetEventHandler) error {
	if opt == nil || opt.ConnectParams == nil {
		return fmt.Errorf("msgque options is nil")
	}
	if err := opt.CheckOptions(); err != nil {
		return err
	}
	xlog.Infof("start to connect [%v-%v][%v]", opt.ConnectParams.SvrType, opt.ConnectParams.SvrId, opt.ConnectParams.ConnectAddr)
	msgque := newTcpConnect(handler, opt)
	msgque.connect(mgr)
	return nil
}

func (mgr *NetMgr) addSess(mq IMsgQue) {
	if mq == nil {
		return
	}
	mgr.sessMap[mq.SessId()] = mq
	mgr.trackLoginPending(mq)

	if mq.GetConnectType() == ConnTypeAccept {
		mgr.incAcceptSessNum()
	}
}

func (mgr *NetMgr) disconnectReason(mq IMsgQue, canReconnect bool) (string, string) {
	cause := "unknown"
	if tcpMq, ok := mq.(*tcpMsgQue); ok {
		if reason := tcpMq.getDisconnectReason(); reason != "" {
			cause = reason
		}
	}
	action := "peer closed"
	switch {
	case mgr.isStopping():
		action = "while stopping"
	case mq.GetConnectType() == ConnTypeConn && canReconnect:
		action = "reconnect scheduled"
	case mq.GetConnectType() == ConnTypeConn:
		action = "target unavailable"
	}
	return cause, action
}

func (mgr *NetMgr) sessOverEvt(mq IMsgQue) {
	mgr.addTask(func() {
		if mq == nil {
			return
		}

		sessId := mq.SessId()

		tc := mgr.sessMap[sessId]
		if tc == nil {
			return
		}

		opt := mq.getOpt()
		svrType, svrId := mq.GetAgent().GetSvrAgt()

		// clear
		mgr.deleteSess(sessId)
		mgr.decSessAgent(svrType, sessId)

		// stop() may block waiting for IO; do it outside NetMgr task loop.
		basal.SafeGo(func() { tc.stop() })
		canReconnect := opt.ConnectParams != nil && mgr.CanReconnect(opt.ConnectParams)
		userId := tc.GetAgent().GetCltUser()
		cause, action := mgr.disconnectReason(tc, canReconnect)
		connType := lo.Ternary(tc.GetConnectType() == ConnTypeAccept, "accept", "conn")
		xlog.Infof("conn disconnect [sessid=%d gid=%d connType=%s peer=%s-%d self=%s-%d remote=%s remoteIp=%s cause=%s action=%s].",
			sessId, userId, connType, svrType, svrId, mgr.localSvrType, mgr.localSvrId,
			tc.remoteAddr(), tc.remoteIP(), cause, action)

		if canReconnect {
			mgr.reconnectWithNewMq(tc)
		}
	})
}

func (mgr *NetMgr) reconnectWithNewMq(mq IMsgQue) {
	if mq == nil {
		return
	}
	opt := mq.getOpt()
	if opt == nil || opt.ConnectParams == nil {
		return
	}
	tcpMq, ok := mq.(*tcpMsgQue)
	if !ok {
		return
	}
	newMq := newTcpConnect(tcpMq.handler, opt)
	newMq.connect(mgr)
}

func (mgr *NetMgr) deleteSess(sessId int64) {
	tc := mgr.sessMap[sessId]
	if tc == nil {
		return
	}
	if tc.GetConnectType() == ConnTypeAccept {
		mgr.decAcceptSessNum()
	}
	delete(mgr.sessMap, sessId)
	mgr.clearLoginPending(sessId)
}

func (mgr *NetMgr) decSessAgent(svrName string, sessId int64) {
	if mgr.sessAgent[svrName] == nil {
		return
	}

	svrId := int32(-1)
	for k, v := range mgr.sessAgent[svrName] {
		if v.SessId() == sessId {
			svrId = k
			break
		}
	}

	if svrId >= 0 {
		delete(mgr.sessAgent[svrName], svrId)
	}
}

func (mgr *NetMgr) trackLoginPending(mq IMsgQue) {
	if !mgr.needsLogin(mq) {
		return
	}
	if mgr.loginPending == nil {
		mgr.loginPending = make(map[int64]struct{})
	}
	mgr.loginPending[mq.SessId()] = struct{}{}
}

func (mgr *NetMgr) clearLoginPending(sessId int64) {
	if mgr.loginPending == nil {
		return
	}
	delete(mgr.loginPending, sessId)
}

func (mgr *NetMgr) needsLogin(mq IMsgQue) bool {
	if mq == nil || mq.GetConnectType() != ConnTypeAccept {
		return false
	}
	agt := mq.GetAgent()
	if agt == nil {
		return true
	}
	if agt.GetCltUser() > 0 {
		return false
	}
	svrType, _ := agt.GetSvrAgt()
	return svrType == ""
}

func (mgr *NetMgr) handleLoginTimeout(now time.Time) {
	if len(mgr.loginPending) == 0 {
		return
	}

	nowMillis := now.UnixMilli()
	timeoutMillis := loginTimeout.Milliseconds()
	for sessId := range mgr.loginPending {
		mq := mgr.sessMap[sessId]
		if mq == nil {
			delete(mgr.loginPending, sessId)
			continue
		}
		if !mgr.needsLogin(mq) {
			delete(mgr.loginPending, sessId)
			continue
		}
		if nowMillis-fastid.GetTimeMillFromFastID(sessId) <= timeoutMillis {
			continue
		}

		mgr.deleteSess(sessId)
		// stop() may block waiting for IO; do it outside NetMgr task loop.
		basal.SafeGo(func() { mq.stop() })
		xlog.Warnf("conn [%d] addr %s login timeout, stop it.", sessId, mq.remoteAddr())
	}
}

// ------------------------------------------------------------------
func (mgr *NetMgr) SendMsg2One(svrType string, msg *msg.Message, failFunc func()) {
	fail := func(reason string) {
		xlog.Infof("send msg[%v] to one server[serverType=%s] failed for reason: %s", msg.MsgId(), svrType, reason)
		if failFunc != nil {
			failFunc()
		}
	}
	if ok := mgr.addTask(func() {
		sp := mgr.sessAgent[svrType]
		if len(sp) == 0 {
			fail("no conn")
			return
		}
		ok := false
		for _, v := range sp {
			if sendOk := v.Send(msg); sendOk {
				ok = true
				break
			}
		}
		if !ok {
			fail(fmt.Sprintf("send failed"))
		}

	}); !ok {
		fail("add task failed")
	}
}

func (mgr *NetMgr) SendMsg2All(svrType string, msg *msg.Message, failFunc func()) {
	fail := func(reason string) {
		xlog.Infof("send msg[%v] to all server[serverType=%s] failed for reason: %s", msg.MsgId(), svrType, reason)
		if failFunc != nil {
			failFunc()
		}
	}
	if ok := mgr.addTask(func() {
		sp := mgr.sessAgent[svrType]
		if len(sp) == 0 {
			fail("no conn")
			return
		}

		sendFailId := int64(0)
		for _, v := range sp {
			if sendOk := v.Send(msg); !sendOk {
				sendFailId = v.SessId()
			}
		}
		if sendFailId > 0 {
			fail(fmt.Sprintf("send failed sessId=%d", sendFailId))
		}
	}); !ok {
		fail("add task failed")
	}
}

func (mgr *NetMgr) SendMsg2Fix(svrType string, svrId int32, msg *msg.Message, failFunc func()) {
	fail := func(reason string) {
		xlog.Infof("send msg[%v] to server[serverType=%s serverId=%d] failed for reason: %s", msg.MsgId(), svrType, svrId, reason)
		if failFunc != nil {
			failFunc()
		}
	}

	if ok := mgr.addTask(func() {
		tc := mgr.sessAgent[svrType][svrId]
		if tc == nil {
			fail("no conn")
			return
		}
		if sendOk := tc.Send(msg); !sendOk {
			fail("send failed")
		}
	}); !ok {
		fail("add task failed")
	}
}

func (mgr *NetMgr) SendMsg2Sess(sessId int64, msg *msg.Message, failFunc func()) {
	fail := func(reason string) {
		xlog.Infof("send msg[%v] to sess[sessId=%d] failed for reason: %s", msg.MsgId(), sessId, reason)
		if failFunc != nil {
			failFunc()
		}
	}

	if ok := mgr.addTask(func() {
		tc := mgr.sessMap[sessId]
		if tc == nil {
			fail("no conn")
			return
		}
		if sendOk := tc.Send(msg); !sendOk {
			fail("send failed")
		}
	}); !ok {
		fail("add task failed")
	}
}

func (mgr *NetMgr) SendMsg2AllUser(msg *msg.Message, failFunc func()) {

	ok := mgr.addTask(func() {
		for _, v := range mgr.sessMap {
			agt := v.GetAgent()
			if agt != nil && agt.GetCltUser() > 0 {
				v.Send(msg)
			}
		}
	})
	if !ok && failFunc != nil {
		failFunc()
	}
}

func NewNetMgr() *NetMgr {
	return &NetMgr{
		listenMap:    map[string]IMsgQue{},
		sessMap:      map[int64]IMsgQue{},
		sessAgent:    map[string]map[int32]IMsgQue{},
		taskChan:     make(chan func(), 100000),
		stopCh:       make(chan struct{}),
		loginPending: map[int64]struct{}{},
	}
}
