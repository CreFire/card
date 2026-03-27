package netmgr

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"game/deps/basal"
	"game/deps/encrypt"
	"game/deps/fastid"
	"game/deps/msg"
	"game/deps/netmgr/options"
	"game/deps/proto/msgbase"
	"game/deps/xlog"
	"game/src/proto/pb"
)

type recordHandler struct {
	newCh     chan IMsgQue
	connectCh chan IMsgQue
	stopCh    chan IMsgQue
	msgCh     chan *msg.Message
}

var benchDelay time.Duration

func newRecordHandler() *recordHandler {
	return &recordHandler{
		newCh:     make(chan IMsgQue, 4),
		connectCh: make(chan IMsgQue, 4),
		stopCh:    make(chan IMsgQue, 4),
		msgCh:     make(chan *msg.Message, 8),
	}
}

func (h *recordHandler) OnConnectSuccess(msgque IMsgQue) bool {
	h.connectCh <- msgque
	return true
}

func (h *recordHandler) OnNewMsgQue(msgque IMsgQue) bool {
	h.newCh <- msgque
	return true
}

func (h *recordHandler) OnMsgQueStop(msgque IMsgQue) {
	h.stopCh <- msgque
}

func (h *recordHandler) OnProcessMsg(msgque IMsgQue, m *msg.Message) bool {
	h.msgCh <- m
	return true
}

func waitForMessage(t *testing.T, ch <-chan *msg.Message, desc string) *msg.Message {
	t.Helper()
	select {
	case m := <-ch:
		return m
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for message: %s", desc)
		return nil
	}
}

func waitForConn(t *testing.T, ch <-chan IMsgQue, desc string) IMsgQue {
	t.Helper()
	select {
	case mq := <-ch:
		return mq
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for conn: %s", desc)
		return nil
	}
}

func waitForClosed(t *testing.T, ch <-chan struct{}, desc string) {
	t.Helper()
	select {
	case <-ch:
		return
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for close: %s", desc)
	}
}

func waitForCondition(t *testing.T, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for condition: %s", desc)
}

func setupMsgQueWithConn(t *testing.T, mgr *NetMgr, handler INetEventHandler, conn net.Conn) *tcpMsgQue {
	t.Helper()
	opt := options.NewMsgQueOptions()
	opt.WriteChanSize = 8
	mq := newTcpConnect(handler, opt)
	mq.conn = conn
	mq.discEvt = mgr.sessOverEvt

	if !mgr.runTaskSync(func() {
		mgr.addSess(mq)
		mq.Start()
	}) {
		t.Fatalf("failed to setup msgque")
	}
	return mq
}

type partialWriteConn struct {
	mu       sync.Mutex
	maxWrite int
	writes   [][]byte
}

func (c *partialWriteConn) Write(p []byte) (int, error) {
	n := len(p)
	if c.maxWrite > 0 && n > c.maxWrite {
		n = c.maxWrite
	}
	buf := make([]byte, n)
	copy(buf, p[:n])
	c.mu.Lock()
	c.writes = append(c.writes, buf)
	c.mu.Unlock()
	return n, nil
}

func (c *partialWriteConn) Read(p []byte) (int, error) { return 0, io.EOF }
func (c *partialWriteConn) Close() error               { return nil }
func (c *partialWriteConn) LocalAddr() net.Addr        { return dummyAddr("local") }
func (c *partialWriteConn) RemoteAddr() net.Addr       { return dummyAddr("remote") }
func (c *partialWriteConn) SetDeadline(t time.Time) error {
	return nil
}
func (c *partialWriteConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (c *partialWriteConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type dummyAddr string

func (d dummyAddr) Network() string { return "dummy" }
func (d dummyAddr) String() string  { return string(d) }

type blockingWriteConn struct {
	closeCh chan struct{}
	wroteCh chan struct{}
	once    sync.Once
}

func newBlockingWriteConn() *blockingWriteConn {
	return &blockingWriteConn{
		closeCh: make(chan struct{}),
		wroteCh: make(chan struct{}),
	}
}

func captureDefaultLog(t *testing.T) func() string {
	t.Helper()

	originalLogger := xlog.DefaultLogger
	logPath := filepath.Join(t.TempDir(), "netmgr.log")
	xlog.DefaultLogger = xlog.NewMyLoggerWithOptions(xlog.Options{
		FilePath: logPath,
		Level:    "debug",
		Skip:     1,
		Sync:     true,
		FileOut:  true,
		StdOut:   false,
	})
	t.Cleanup(func() {
		xlog.DefaultLogger.Close()
		xlog.DefaultLogger = originalLogger
	})
	return func() string {
		_ = xlog.Sync()
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read log file failed: %v", err)
		}
		return string(data)
	}
}

func (c *blockingWriteConn) Write(p []byte) (int, error) {
	c.once.Do(func() { close(c.wroteCh) })
	<-c.closeCh
	return 0, io.ErrClosedPipe
}

func (c *blockingWriteConn) Read(p []byte) (int, error) {
	<-c.closeCh
	return 0, io.EOF
}

func (c *blockingWriteConn) Close() error {
	select {
	case <-c.closeCh:
	default:
		close(c.closeCh)
	}
	return nil
}

func (c *blockingWriteConn) LocalAddr() net.Addr  { return dummyAddr("local") }
func (c *blockingWriteConn) RemoteAddr() net.Addr { return dummyAddr("remote") }
func (c *blockingWriteConn) SetDeadline(t time.Time) error {
	return nil
}
func (c *blockingWriteConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (c *blockingWriteConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type writeReasonConn struct {
	closeCh     chan struct{}
	readUnblock chan struct{}
	wroteCh     chan struct{}
	writeOnce   sync.Once
	readOnce    sync.Once
	closeOnce   sync.Once
}

func newWriteReasonConn() *writeReasonConn {
	return &writeReasonConn{
		closeCh:     make(chan struct{}),
		readUnblock: make(chan struct{}),
		wroteCh:     make(chan struct{}),
	}
}

func (c *writeReasonConn) Write(p []byte) (int, error) {
	c.writeOnce.Do(func() { close(c.wroteCh) })
	<-c.closeCh
	return 0, io.ErrClosedPipe
}

func (c *writeReasonConn) Read(p []byte) (int, error) {
	<-c.readUnblock
	return 0, io.EOF
}

func (c *writeReasonConn) Close() error {
	c.closeOnce.Do(func() { close(c.closeCh) })
	return nil
}

func (c *writeReasonConn) CloseRead() error {
	c.readOnce.Do(func() { close(c.readUnblock) })
	return nil
}

func (c *writeReasonConn) LocalAddr() net.Addr  { return dummyAddr("local") }
func (c *writeReasonConn) RemoteAddr() net.Addr { return dummyAddr("remote") }

func (c *writeReasonConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *writeReasonConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *writeReasonConn) SetWriteDeadline(t time.Time) error { return nil }

type blockingReadConn struct {
	started     chan struct{}
	unblock     chan struct{}
	startOnce   sync.Once
	unblockOnce sync.Once
}

func newBlockingReadConn() *blockingReadConn {
	return &blockingReadConn{
		started: make(chan struct{}),
		unblock: make(chan struct{}),
	}
}

func (c *blockingReadConn) Read(p []byte) (int, error) {
	c.startOnce.Do(func() { close(c.started) })
	<-c.unblock
	return 0, io.EOF
}

func (c *blockingReadConn) Write(p []byte) (int, error) { return len(p), nil }

func (c *blockingReadConn) Close() error {
	c.unblockOnce.Do(func() { close(c.unblock) })
	return nil
}

func (c *blockingReadConn) CloseRead() error { return nil }

func (c *blockingReadConn) LocalAddr() net.Addr  { return dummyAddr("local") }
func (c *blockingReadConn) RemoteAddr() net.Addr { return dummyAddr("remote") }

func (c *blockingReadConn) SetDeadline(t time.Time) error {
	return c.SetReadDeadline(t)
}

func (c *blockingReadConn) SetReadDeadline(t time.Time) error {
	c.unblockOnce.Do(func() { close(c.unblock) })
	return nil
}

func (c *blockingReadConn) SetWriteDeadline(t time.Time) error { return nil }

func buildMsg(msgID pb.MSG_ID, data []byte, flags int32) *msg.Message {
	m := msg.NewMsg(msgID, data)
	if m.Head != nil {
		m.Head.Flags = flags
	}
	return m
}

func writeFrames(t *testing.T, conn net.Conn, msgs ...*msg.Message) {
	t.Helper()
	buffer := &bytes.Buffer{}
	for _, m := range msgs {
		if _, err := m.Bytes(buffer); err != nil {
			t.Fatalf("build frame failed: %v", err)
		}
	}
	if _, err := conn.Write(buffer.Bytes()); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write frame failed: %v", err)
	}
}

type stubMsgQue struct {
	sessId  int64
	connTyp ConnType
	agt     *ConnAgt
	opt     *options.NetOptions
	stopped atomic.Bool
	sent    []*msg.Message
	sendRet bool
	hasRet  bool
}

func (s *stubMsgQue) SessId() int64      { return s.sessId }
func (s *stubMsgQue) GetAgent() *ConnAgt { return s.agt }
func (s *stubMsgQue) Send(m *msg.Message) (re bool) {
	s.sent = append(s.sent, m)
	if s.hasRet {
		return s.sendRet
	}
	return true
}
func (s *stubMsgQue) listen(mgr *NetMgr)             {}
func (s *stubMsgQue) connect(mgr *NetMgr)            {}
func (s *stubMsgQue) stop()                          { s.stopped.Store(true) }
func (s *stubMsgQue) GetConnectType() ConnType       { return s.connTyp }
func (s *stubMsgQue) remoteAddr() string             { return "stub" }
func (s *stubMsgQue) remoteIP() string               { return "stub" }
func (s *stubMsgQue) getOpt() *options.NetOptions    { return s.opt }
func (s *stubMsgQue) getDhKey() []byte               { return nil }
func (s *stubMsgQue) setDhKey(key []byte)            {}
func (s *stubMsgQue) setOpt(opt *options.NetOptions) { s.opt = opt }

func drainTasks(mgr *NetMgr) {
	for {
		select {
		case f := <-mgr.taskChan:
			f()
		default:
			return
		}
	}
}

func TestTcpMsgQueHandshakeSharedSecret(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	opt := options.NewMsgQueOptions()
	server := &tcpMsgQue{msgQue: msgQue{opt: opt}, quit: make(chan struct{})}
	client := &tcpMsgQue{msgQue: msgQue{opt: opt}, quit: make(chan struct{})}

	type result struct {
		key []byte
		err error
	}
	serverCh := make(chan result, 1)
	clientCh := make(chan result, 1)

	go func() {
		key, err := server.dhKeyExchange(c1)
		serverCh <- result{key: key, err: err}
	}()
	go func() {
		key, err := client.dhKeyExchangeC(c2)
		clientCh <- result{key: key, err: err}
	}()

	sr := <-serverCh
	cr := <-clientCh

	if sr.err != nil || cr.err != nil {
		t.Fatalf("handshake failed: server=%v client=%v", sr.err, cr.err)
	}
	if !bytes.Equal(sr.key, cr.key) {
		t.Fatalf("shared secret mismatch")
	}
}

func TestReadStateDefaultSize(t *testing.T) {
	state := newReadState(0)
	if state == nil {
		t.Fatalf("state is nil")
	}
	if len(state.buf) != options.DEFAULT_BUFF_SIZE {
		t.Fatalf("default buffer size mismatch: got %d", len(state.buf))
	}
}

func TestReadStateResetFrame(t *testing.T) {
	state := newReadState(16)
	state.headLen = 8
	state.bodyLen = 4
	state.msg = &msg.Message{}

	state.resetFrame()
	if state.headLen != 0 || state.bodyLen != 0 || state.msg != nil {
		t.Fatalf("resetFrame did not clear state")
	}
}

func TestReadStateGrowPreservesData(t *testing.T) {
	state := &readState{buf: make([]byte, 4)}
	copy(state.buf, []byte{1, 2, 3, 4})
	state.offset = 4

	state.grow(8)
	if len(state.buf) != 8 {
		t.Fatalf("buffer not grown")
	}
	if !bytes.Equal(state.buf[:4], []byte{1, 2, 3, 4}) {
		t.Fatalf("buffer data not preserved")
	}
}

func TestReadFromConnAppendsToBuffer(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	opt := options.NewMsgQueOptions()
	mq := &tcpMsgQue{msgQue: msgQue{opt: opt}, conn: c1, quit: make(chan struct{})}
	state := newReadState(8)
	state.buf[0] = 'x'
	state.buf[1] = 'y'
	state.offset = 2

	type result struct {
		n   int
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		n, err := mq.readFromConn(state)
		resCh <- result{n: n, err: err}
	}()

	if _, err := c2.Write([]byte("abc")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	res := <-resCh
	if res.err != nil {
		t.Fatalf("read failed: %v", res.err)
	}
	if res.n != 3 {
		t.Fatalf("unexpected read size: %d", res.n)
	}
	if state.offset != 5 {
		t.Fatalf("unexpected offset: %d", state.offset)
	}
	if string(state.buf[:5]) != "xyabc" {
		t.Fatalf("unexpected buffer data: %q", string(state.buf[:5]))
	}
}

func TestTcpMsgQueReadMultipleMessages(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	opt := options.NewMsgQueOptions()
	opt.ReadSize = 256
	opt.WriteChanSize = 8

	handler := newRecordHandler()
	mq := newTcpAccept(c1, handler, opt)
	mq.Start()
	defer mq.stop()

	msg1 := buildMsg(pb.MSG_ID(1), []byte("one"), 0)
	msg2 := buildMsg(pb.MSG_ID(2), []byte("two"), 0)
	writeFrames(t, c2, msg1, msg2)

	got1 := waitForMessage(t, handler.msgCh, "first")
	if got1.MsgId() != int32(msg1.MsgId()) || string(got1.Data) != "one" {
		t.Fatalf("first message mismatch")
	}
	got2 := waitForMessage(t, handler.msgCh, "second")
	if got2.MsgId() != int32(msg2.MsgId()) || string(got2.Data) != "two" {
		t.Fatalf("second message mismatch")
	}
}

func TestTcpMsgQueReadAcceptAllowsGrowthByDefault(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	opt := options.NewMsgQueOptions()
	opt.ReadSize = 64
	opt.WriteChanSize = 8

	handler := newRecordHandler()
	mq := newTcpAccept(c1, handler, opt)
	mq.Start()
	defer mq.stop()

	payload := bytes.Repeat([]byte("a"), 100)
	writeFrames(t, c2, buildMsg(pb.MSG_ID(1), payload, 0))

	got := waitForMessage(t, handler.msgCh, "accept growth")
	if got.MsgId() != int32(pb.MSG_ID(1)) || len(got.Data) != len(payload) {
		t.Fatalf("accept message mismatch")
	}
}

func TestTcpMsgQueReadGateExternalTooLarge(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	opt := options.NewMsgQueOptions()
	opt.ReadSize = 64
	opt.WriteChanSize = 8
	opt.SetIsGate(true)

	mq := newTcpAccept(c1, nil, opt)
	mq.Start()
	defer mq.stop()

	payload := bytes.Repeat([]byte("a"), 100)
	writeFrames(t, c2, buildMsg(pb.MSG_ID(1), payload, 0))

	waitForClosed(t, mq.quit, "gate external too large")
}

func TestTcpMsgQueReadInternalAllowsGrowth(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	opt := options.NewMsgQueOptions()
	opt.ReadSize = 64
	opt.WriteChanSize = 8

	handler := newRecordHandler()
	mq := newTcpConnect(handler, opt)
	mq.conn = c1
	mq.Start()
	defer mq.stop()

	payload := bytes.Repeat([]byte("b"), 150)
	writeFrames(t, c2, buildMsg(pb.MSG_ID(1), payload, 0))

	got := waitForMessage(t, handler.msgCh, "internal growth")
	if got.MsgId() != int32(pb.MSG_ID(1)) || len(got.Data) != len(payload) {
		t.Fatalf("internal message mismatch")
	}
}

func TestTcpMsgQueReadInternalTooLarge(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c2.Close()

	opt := options.NewMsgQueOptions()
	opt.ReadSize = 64
	opt.WriteChanSize = 8

	mq := newTcpConnect(nil, opt)
	mq.conn = c1
	mq.Start()
	defer mq.stop()

	payload := bytes.Repeat([]byte("c"), 300)
	writeFrames(t, c2, buildMsg(pb.MSG_ID(1), payload, 0))

	waitForClosed(t, mq.quit, "internal too large")
}

func TestTcpMsgQueHeadLenTooLarge(t *testing.T) {
	opt := options.NewMsgQueOptions()
	mq := &tcpMsgQue{msgQue: msgQue{opt: opt}, quit: make(chan struct{})}
	state := newReadState(opt.ReadSize)

	headLen := uint16(MAX_HEAD_LEN + 1)
	state.buf[0] = byte(headLen >> 8)
	state.buf[1] = byte(headLen)
	state.offset = HEAD_SIZE

	if mq.consumeReadBuffer(state) {
		t.Fatalf("expected head len validation to fail")
	}
}

func TestTcpMsgQueConsumeReadBufferPartial(t *testing.T) {
	opt := options.NewMsgQueOptions()
	opt.ReadSize = 128
	handler := newRecordHandler()
	mq := &tcpMsgQue{msgQue: msgQue{opt: opt, handler: handler, connTyp: ConnTypeAccept, agt: newConnAgt()}, quit: make(chan struct{})}

	msg1 := buildMsg(pb.MSG_ID(1), []byte("hello"), 0)
	buf := &bytes.Buffer{}
	if _, err := msg1.Bytes(buf); err != nil {
		t.Fatalf("build frame failed: %v", err)
	}
	frame := buf.Bytes()

	state := newReadState(opt.ReadSize)
	copy(state.buf, frame[:len(frame)/2])
	state.offset = len(frame) / 2

	if !mq.consumeReadBuffer(state) {
		t.Fatalf("partial consume should not fail")
	}
	select {
	case <-handler.msgCh:
		t.Fatalf("message should not be processed yet")
	default:
	}

	copy(state.buf[state.offset:], frame[len(frame)/2:])
	state.offset += len(frame) - len(frame)/2

	if !mq.consumeReadBuffer(state) {
		t.Fatalf("full consume should succeed")
	}
	got := waitForMessage(t, handler.msgCh, "partial complete")
	if string(got.Data) != "hello" {
		t.Fatalf("message data mismatch")
	}
}

func TestTcpMsgQueFlushWriteBufferPartialWrites(t *testing.T) {
	conn := &partialWriteConn{maxWrite: 3}
	opt := options.NewMsgQueOptions()
	mq := &tcpMsgQue{msgQue: msgQue{opt: opt}, conn: conn, quit: make(chan struct{})}

	buffer := &bytes.Buffer{}
	buffer.WriteString("abcdef")

	if err := mq.flushWriteBuffer(buffer); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if buffer.Len() != 0 {
		t.Fatalf("buffer not reset")
	}

	var combined []byte
	for _, w := range conn.writes {
		combined = append(combined, w...)
	}
	if string(combined) != "abcdef" {
		t.Fatalf("partial writes did not flush full data")
	}
}

func TestTcpMsgQueAdaptiveWriteDelayDisabled(t *testing.T) {
	opt := options.NewMsgQueOptions()
	opt.DelayWrite = 10
	mq := &tcpMsgQue{msgQue: msgQue{opt: opt}, delayCtrl: newTcpWriteDelayCtrl(opt.DelayWrite, WriteChanLowWatermark), quit: make(chan struct{})}

	if d := mq.adaptiveWriteDelay(0, false); d != 0 {
		t.Fatalf("expected no delay when allowDelay is false, got %v", d)
	}

	optZero := options.NewMsgQueOptions()
	optZero.DelayWrite = 0
	mqZero := &tcpMsgQue{msgQue: msgQue{opt: optZero}, delayCtrl: newTcpWriteDelayCtrl(optZero.DelayWrite, WriteChanLowWatermark), quit: make(chan struct{})}
	if d := mqZero.adaptiveWriteDelay(0, true); d != 0 {
		t.Fatalf("expected no delay when DelayWrite is disabled, got %v", d)
	}
}

func TestTcpMsgQueAdaptiveWriteDelayAimd(t *testing.T) {
	opt := options.NewMsgQueOptions()
	opt.DelayWrite = 8
	mq := &tcpMsgQue{msgQue: msgQue{opt: opt}, delayCtrl: newTcpWriteDelayCtrl(opt.DelayWrite, WriteChanLowWatermark), quit: make(chan struct{})}
	highThreshold := WriteChanLowWatermark / 3
	if highThreshold <= 0 {
		highThreshold = 1
	}

	if d := mq.adaptiveWriteDelay(0, true); d != 8*time.Millisecond {
		t.Fatalf("unexpected initial delay: %v", d)
	}

	highBatch := WriteChanLowWatermark + 1
	for range highThreshold - 1 {
		if d := mq.adaptiveWriteDelay(highBatch, true); d != 8*time.Millisecond {
			t.Fatalf("unexpected delay before high streak threshold: %v", d)
		}
	}
	if d := mq.adaptiveWriteDelay(highBatch, true); d != 4*time.Millisecond {
		t.Fatalf("expected delay to halve after high streak, got %v", d)
	}

	for range highThreshold - 1 {
		if d := mq.adaptiveWriteDelay(highBatch, true); d != 4*time.Millisecond {
			t.Fatalf("unexpected delay before second high streak threshold: %v", d)
		}
	}
	if d := mq.adaptiveWriteDelay(highBatch, true); d != 2*time.Millisecond {
		t.Fatalf("expected delay to halve again after high streak, got %v", d)
	}

	lowBatch := WriteChanLowWatermark
	for range WriteChanLowWatermark - 1 {
		if d := mq.adaptiveWriteDelay(lowBatch, true); d != 2*time.Millisecond {
			t.Fatalf("unexpected delay before low streak threshold: %v", d)
		}
	}
	if d := mq.adaptiveWriteDelay(lowBatch, true); d != 3*time.Millisecond {
		t.Fatalf("expected delay to increase slowly after low streak, got %v", d)
	}
}

func TestTcpMsgQueCollectWriteBatchTracksLastBatch(t *testing.T) {
	opt := options.NewMsgQueOptions()
	mq := &tcpMsgQue{
		msgQue: msgQue{
			opt:    opt,
			cwrite: make(chan *msg.Message, 4),
		},
		delayCtrl: newTcpWriteDelayCtrl(opt.DelayWrite, WriteChanLowWatermark),
		quit:      make(chan struct{}),
	}
	buffer := &bytes.Buffer{}

	first := buildMsg(pb.MSG_ID(1), []byte("a"), 0)
	mq.cwrite <- buildMsg(pb.MSG_ID(2), []byte("b"), 0)
	mq.cwrite <- buildMsg(pb.MSG_ID(3), []byte("c"), 0)

	if err := mq.collectWriteBatch(buffer, first, false); err != nil {
		t.Fatalf("collectWriteBatch failed: %v", err)
	}
	if mq.lastWriteBatch != 3 {
		t.Fatalf("unexpected lastWriteBatch: %d", mq.lastWriteBatch)
	}
	if len(mq.cwrite) != 0 {
		t.Fatalf("expected write queue to be drained, got %d", len(mq.cwrite))
	}
}

func TestTcpMsgQueIsInternalConn(t *testing.T) {
	internal := &tcpMsgQue{msgQue: msgQue{connTyp: ConnTypeConn, agt: newConnAgt()}, quit: make(chan struct{})}
	if !internal.isInternalConn() {
		t.Fatalf("ConnTypeConn should be internal")
	}

	acceptInternal := &tcpMsgQue{msgQue: msgQue{connTyp: ConnTypeAccept, agt: newConnAgt()}, quit: make(chan struct{})}
	if !acceptInternal.isInternalConn() {
		t.Fatalf("non-gate accept should be internal")
	}

	gateExternal := &tcpMsgQue{
		msgQue: msgQue{
			connTyp: ConnTypeAccept,
			agt:     newConnAgt(),
			opt:     (&options.NetOptions{}).SetIsGate(true),
		},
		quit: make(chan struct{}),
	}
	gateExternal.agt.AddSvrAgt("logic", 1)
	if gateExternal.isInternalConn() {
		t.Fatalf("gate listen connections should stay external")
	}
}

func TestMsgQueSendChannelFull(t *testing.T) {
	mq := &msgQue{
		cwrite: make(chan *msg.Message, 1),
		agt:    newConnAgt(),
	}

	if !mq.Send(msg.NewMsg(pb.MSG_ID(1), []byte("a"))) {
		t.Fatalf("first send should succeed")
	}
	if mq.Send(msg.NewMsg(pb.MSG_ID(2), []byte("b"))) {
		t.Fatalf("second send should fail when channel is full")
	}
}

func TestMsgQueCompressOrEncryptCompresses(t *testing.T) {
	opt := options.NewMsgQueOptions()
	opt.IsGate = true
	opt.Compress = true
	opt.CompressMode = NET_COMPRESS_MODE_GZIP
	opt.CompressLimit = 1

	mq := &tcpMsgQue{msgQue: msgQue{opt: opt}, quit: make(chan struct{})}
	originalData := bytes.Repeat([]byte("a"), 1024)
	m := msg.NewMsg(pb.MSG_ID(1), originalData)

	out := mq.CompressOrEncrypt(m)
	if out == m {
		t.Fatalf("expected new message when compression is effective")
	}
	if out.Head.Flags&msg.FlagCompress == 0 {
		t.Fatalf("compress flag not set")
	}
	if len(out.Data) >= len(originalData) {
		t.Fatalf("compressed data not smaller")
	}
	if !bytes.Equal(m.Data, originalData) {
		t.Fatalf("original data mutated")
	}
}

func TestMsgQueCompressOrEncryptNoBenefit(t *testing.T) {
	opt := options.NewMsgQueOptions()
	opt.IsGate = true
	opt.Compress = true
	opt.CompressMode = NET_COMPRESS_MODE_GZIP
	opt.CompressLimit = 1

	mq := &tcpMsgQue{msgQue: msgQue{opt: opt}, quit: make(chan struct{})}
	data := []byte("abcdefg")
	m := msg.NewMsg(pb.MSG_ID(1), data)

	out := mq.CompressOrEncrypt(m)
	if out != m {
		t.Fatalf("expected original message when compression is not effective")
	}
}

func TestMsgQueCompressOrEncryptSkipAlreadyCompressed(t *testing.T) {
	opt := options.NewMsgQueOptions()
	opt.IsGate = true
	opt.Compress = true
	opt.CompressMode = NET_COMPRESS_MODE_GZIP
	opt.CompressLimit = 1

	mq := &tcpMsgQue{msgQue: msgQue{opt: opt}, quit: make(chan struct{})}
	originalData := bytes.Repeat([]byte("a"), 1024)
	compressedData := basal.GZipCompress(originalData)
	m := msg.NewMsg(pb.MSG_ID(1), compressedData)
	m.Head.Flags |= msg.FlagCompress

	out := mq.CompressOrEncrypt(m)
	if out != m {
		t.Fatalf("expected already compressed message to be sent as-is")
	}
	if !bytes.Equal(out.Data, compressedData) {
		t.Fatalf("compressed payload should stay unchanged")
	}
}

func TestMsgQueProcessMsgDecompress(t *testing.T) {
	opt := options.NewMsgQueOptions()
	opt.CompressMode = NET_COMPRESS_MODE_GZIP

	handler := newRecordHandler()
	mq := &tcpMsgQue{
		msgQue: msgQue{
			connTyp: ConnTypeAccept,
			handler: handler,
			agt:     newConnAgt(),
			opt:     opt,
		},
		quit: make(chan struct{}),
	}

	original := bytes.Repeat([]byte("a"), 256)
	compressed := basal.GZipCompress(original)

	m := &msg.Message{
		Head: &msgbase.MsgHead{
			MsgId:   pb.MSG_ID(1),
			BodyLen: int32(len(compressed)),
			Flags:   msg.FlagCompress,
		},
		Data: compressed,
	}

	if !mq.processMsg(mq, m) {
		t.Fatalf("process should succeed")
	}
	got := waitForMessage(t, handler.msgCh, "decompress")
	if !bytes.Equal(got.Data, original) {
		t.Fatalf("decompressed data mismatch")
	}
	if got.Head.BodyLen != int32(len(original)) {
		t.Fatalf("body len not updated")
	}
}

func TestMsgQueProcessMsgDecrypt(t *testing.T) {
	opt := options.NewMsgQueOptions()
	handler := newRecordHandler()
	mq := &tcpMsgQue{
		msgQue: msgQue{
			connTyp: ConnTypeAccept,
			handler: handler,
			agt:     newConnAgt(),
			opt:     opt,
		},
		quit: make(chan struct{}),
	}

	key := bytes.Repeat([]byte{1}, 32)
	plain := []byte("secret")
	encrypted, err := encrypt.AesEncodeData(plain, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	mq.dhKey = key

	m := &msg.Message{
		Head: &msgbase.MsgHead{
			MsgId:   pb.MSG_ID(1),
			BodyLen: int32(len(encrypted)),
			Flags:   msg.FlagEncrypt,
		},
		Data: encrypted,
	}

	if !mq.processMsg(mq, m) {
		t.Fatalf("process should succeed")
	}
	got := waitForMessage(t, handler.msgCh, "decrypt")
	if !bytes.Equal(got.Data, plain) {
		t.Fatalf("decrypted data mismatch")
	}
	if got.Head.Flags&msg.FlagEncrypt != 0 {
		t.Fatalf("encrypt flag not cleared")
	}
}

func TestNetMgrRegisterSess(t *testing.T) {
	mgr := NewNetMgr()
	mq := &stubMsgQue{
		sessId:  101,
		connTyp: ConnTypeAccept,
		agt:     newConnAgt(),
		opt:     options.NewMsgQueOptions(),
	}

	mgr.sessMap[mq.sessId] = mq
	mgr.loginPending[mq.sessId] = struct{}{}

	mgr.RegisterSess("logic", 2, mq.sessId)
	drainTasks(mgr)

	if _, ok := mgr.sessAgent["logic"][2]; !ok {
		t.Fatalf("session agent not registered")
	}
	if _, ok := mgr.loginPending[mq.sessId]; ok {
		t.Fatalf("login pending not cleared")
	}
	svrType, svrId := mq.agt.GetSvrAgt()
	if svrType != "logic" || svrId != 2 {
		t.Fatalf("agent not updated")
	}
}

func TestNetMgrHandleLoginTimeout(t *testing.T) {
	mgr := NewNetMgr()
	opt := options.NewMsgQueOptions()

	oldMillis := time.Now().Add(-loginTimeout - time.Second).UnixMilli()
	sessId := fastid.MinFastIdAt(oldMillis)
	mq := &stubMsgQue{
		sessId:  sessId,
		connTyp: ConnTypeAccept,
		agt:     newConnAgt(),
		opt:     opt,
	}

	mgr.sessMap[sessId] = mq
	mgr.loginPending[sessId] = struct{}{}

	mgr.handleLoginTimeout(time.Now())

	if _, ok := mgr.sessMap[sessId]; ok {
		t.Fatalf("session not removed on timeout")
	}
	if _, ok := mgr.loginPending[sessId]; ok {
		t.Fatalf("login pending not removed on timeout")
	}
	waitForCondition(t, "session stopped on timeout", func() bool { return mq.stopped.Load() })
}

func TestNetMgrHandleLoginTimeoutWithinLimit(t *testing.T) {
	mgr := NewNetMgr()
	opt := options.NewMsgQueOptions()

	recentMillis := time.Now().Add(-time.Second).UnixMilli()
	sessId := fastid.MinFastIdAt(recentMillis)
	mq := &stubMsgQue{
		sessId:  sessId,
		connTyp: ConnTypeAccept,
		agt:     newConnAgt(),
		opt:     opt,
	}

	mgr.sessMap[sessId] = mq
	mgr.loginPending[sessId] = struct{}{}

	mgr.handleLoginTimeout(time.Now())

	if _, ok := mgr.sessMap[sessId]; !ok {
		t.Fatalf("session removed unexpectedly")
	}
	if _, ok := mgr.loginPending[sessId]; !ok {
		t.Fatalf("login pending removed unexpectedly")
	}
	if mq.stopped.Load() {
		t.Fatalf("session stopped unexpectedly")
	}
}

func TestNetMgrStopFrontKeepsConnSessions(t *testing.T) {
	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	acceptSess := &stubMsgQue{
		sessId:  1,
		connTyp: ConnTypeAccept,
		agt:     newConnAgt(),
	}
	connSess := &stubMsgQue{
		sessId:  2,
		connTyp: ConnTypeConn,
		agt:     newConnAgt(),
	}

	mgr.sessMap[acceptSess.sessId] = acceptSess
	mgr.sessMap[connSess.sessId] = connSess
	mgr.incAcceptSessNum()

	mgr.StopFront()

	if !acceptSess.stopped.Load() {
		t.Fatalf("accept session should be stopped")
	}
	if connSess.stopped.Load() {
		t.Fatalf("conn session should not be stopped")
	}
	if _, ok := mgr.sessMap[acceptSess.sessId]; ok {
		t.Fatalf("accept session should be removed")
	}
	if _, ok := mgr.sessMap[connSess.sessId]; !ok {
		t.Fatalf("conn session should remain")
	}
	if mgr.GetAcceptSessNum() != 0 {
		t.Fatalf("accept session count should be zero")
	}
}

func TestNetMgrStopFrontRejectsNewConnections(t *testing.T) {
	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	opt := options.NewMsgQueOptions()
	opt.SetListenParams(options.NewListenParams("127.0.0.1:0"))

	if err := mgr.StartListen(opt, handler); err != nil {
		t.Fatalf("start listen failed: %v", err)
	}
	mqListen := mgr.listenMap[opt.ListenParams.ListenAddr].(*tcpMsgQue)
	addr := mqListen.listener.Addr().String()

	mgr.StopFront()
	time.Sleep(50 * time.Millisecond)

	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("expected connection to be rejected after StopFront")
	}
}

func TestNetMgrTrackLoginPending(t *testing.T) {
	mgr := NewNetMgr()

	acceptSess := &stubMsgQue{sessId: 1, connTyp: ConnTypeAccept}
	mgr.addSess(acceptSess)

	if _, ok := mgr.loginPending[acceptSess.sessId]; !ok {
		t.Fatalf("accept session should be login pending")
	}

	connSess := &stubMsgQue{sessId: 2, connTyp: ConnTypeConn, agt: newConnAgt()}
	mgr.addSess(connSess)

	if _, ok := mgr.loginPending[connSess.sessId]; ok {
		t.Fatalf("conn session should not be login pending")
	}
}

func TestNetMgrKickSessionMatchGid(t *testing.T) {
	mgr := NewNetMgr()
	agt := newConnAgt()
	agt.AddCltUser(10)

	mq := &stubMsgQue{
		sessId:  100,
		connTyp: ConnTypeAccept,
		agt:     agt,
	}
	mgr.sessMap[mq.sessId] = mq

	mgr.KickSession(mq.sessId, 10)
	drainTasks(mgr)

	if _, ok := mgr.sessMap[mq.sessId]; ok {
		t.Fatalf("session should be removed")
	}
	waitForCondition(t, "session stopped after kick", func() bool { return mq.stopped.Load() })
}

func TestNetMgrKickSessionMismatchGid(t *testing.T) {
	mgr := NewNetMgr()
	agt := newConnAgt()
	agt.AddCltUser(10)

	mq := &stubMsgQue{
		sessId:  100,
		connTyp: ConnTypeAccept,
		agt:     agt,
	}
	mgr.sessMap[mq.sessId] = mq

	mgr.KickSession(mq.sessId, 11)
	drainTasks(mgr)

	if _, ok := mgr.sessMap[mq.sessId]; !ok {
		t.Fatalf("session should remain")
	}
	if mq.stopped.Load() {
		t.Fatalf("session should not be stopped")
	}
}

func TestNetMgrKickSessionZeroGid(t *testing.T) {
	mgr := NewNetMgr()
	agt := newConnAgt()
	agt.AddCltUser(10)

	mq := &stubMsgQue{
		sessId:  100,
		connTyp: ConnTypeAccept,
		agt:     agt,
	}
	mgr.sessMap[mq.sessId] = mq

	mgr.KickSession(mq.sessId, 0)
	drainTasks(mgr)

	if _, ok := mgr.sessMap[mq.sessId]; ok {
		t.Fatalf("session should be removed")
	}
	waitForCondition(t, "session stopped after zero gid kick", func() bool { return mq.stopped.Load() })
}

func TestNetMgrRemoveSvr(t *testing.T) {
	mgr := NewNetMgr()
	agt := newConnAgt()
	agt.AddSvrAgt("logic", 2)

	mq := &stubMsgQue{
		sessId:  200,
		connTyp: ConnTypeConn,
		agt:     agt,
	}
	mgr.sessMap[mq.sessId] = mq
	mgr.sessAgent["logic"] = map[int32]IMsgQue{2: mq}

	mgr.RemoveSvr("logic", 2)
	drainTasks(mgr)

	if _, ok := mgr.sessMap[mq.sessId]; ok {
		t.Fatalf("session should be removed")
	}
	waitForCondition(t, "session stopped after remove svr", func() bool { return mq.stopped.Load() })
	if _, ok := mgr.sessAgent["logic"][2]; ok {
		t.Fatalf("sessAgent entry should be removed")
	}
}

func TestNetMgrAddTaskFull(t *testing.T) {
	mgr := &NetMgr{
		taskChan: make(chan func(), 1),
		stopCh:   make(chan struct{}),
	}
	mgr.taskChan <- func() {}

	if mgr.addTask(func() {}) {
		t.Fatalf("addTask should fail when channel is full")
	}
}

func TestNetMgrRunTaskSyncAfterStop(t *testing.T) {
	mgr := &NetMgr{
		taskChan: make(chan func(), 1),
		stopCh:   make(chan struct{}),
	}
	mgr.Stop()

	var called atomic.Bool
	ok := mgr.runTaskSync(func() { called.Store(true) })
	if !ok {
		t.Fatalf("runTaskSync should return true after stop")
	}
	if !called.Load() {
		t.Fatalf("task should run after stop")
	}
}

func TestNetMgrAddTaskAfterStopDoesNotQueue(t *testing.T) {
	const attempts = 128

	mgr := &NetMgr{
		taskChan: make(chan func(), attempts),
		stopCh:   make(chan struct{}),
	}
	mgr.Stop()

	for i := 0; i < attempts; i++ {
		if mgr.addTask(func() {}) {
			t.Fatalf("addTask should fail after stop")
		}
	}
	if got := len(mgr.taskChan); got != 0 {
		t.Fatalf("addTask queued %d tasks after stop", got)
	}
}

func TestNetMgrStopStateSemantics(t *testing.T) {
	msgData := msg.NewMsg(pb.MSG_ID(1), []byte("a"))
	tests := []struct {
		name string
		send func(mgr *NetMgr, fail func())
	}{
		{
			name: "SendMsg2One",
			send: func(mgr *NetMgr, fail func()) {
				mgr.SendMsg2One("logic", msgData, fail)
			},
		},
		{
			name: "SendMsg2All",
			send: func(mgr *NetMgr, fail func()) {
				mgr.SendMsg2All("logic", msgData, fail)
			},
		},
		{
			name: "SendMsg2Fix",
			send: func(mgr *NetMgr, fail func()) {
				mgr.SendMsg2Fix("logic", 1, msgData, fail)
			},
		},
		{
			name: "SendMsg2Sess",
			send: func(mgr *NetMgr, fail func()) {
				mgr.SendMsg2Sess(1, msgData, fail)
			},
		},
		{
			name: "SendMsg2AllUser",
			send: func(mgr *NetMgr, fail func()) {
				mgr.SendMsg2AllUser(msgData, fail)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewNetMgr()
			if mgr.isStopping() {
				t.Fatalf("mgr should not be stopping before Stop")
			}

			mgr.Stop()
			if !mgr.isStopping() {
				t.Fatalf("mgr should report stopping after Stop")
			}

			called := make(chan struct{}, 1)
			tt.send(mgr, func() {
				called <- struct{}{}
			})

			select {
			case <-called:
			default:
				t.Fatalf("fail callback should be called after Stop")
			}

			if got := len(mgr.taskChan); got != 0 {
				t.Fatalf("send queued %d tasks after Stop", got)
			}
		})
	}
}

func TestNetMgrStopDrainsQueuedTasks(t *testing.T) {
	const attempts = 32

	for i := 0; i < attempts; i++ {
		mgr := NewNetMgr()
		mgr.Start()

		blockStarted := make(chan struct{})
		blockRelease := make(chan struct{})
		if !mgr.addTask(func() {
			close(blockStarted)
			<-blockRelease
		}) {
			t.Fatalf("failed to queue blocking task")
		}
		waitForClosed(t, blockStarted, "blocking task started")

		taskRan := make(chan struct{})
		if !mgr.addTask(func() {
			close(taskRan)
		}) {
			t.Fatalf("failed to queue drain task")
		}

		waitForCondition(t, "drain task queued", func() bool {
			return len(mgr.taskChan) == 1
		})

		stopDone := make(chan struct{})
		go func() {
			mgr.Stop()
			close(stopDone)
		}()

		waitForCondition(t, "mgr stopping", func() bool {
			return mgr.isStopping()
		})

		close(blockRelease)
		waitForClosed(t, stopDone, "stop complete")
		waitForClosed(t, taskRan, "runTaskSync task ran")
	}
}

func TestNetMgrSendMsg2OneNoConn(t *testing.T) {
	mgr := NewNetMgr()
	called := make(chan struct{}, 1)

	mgr.SendMsg2One("logic", msg.NewMsg(pb.MSG_ID(1), []byte("x")), func() {
		called <- struct{}{}
	})
	drainTasks(mgr)

	select {
	case <-called:
	default:
		t.Fatalf("fail callback not called")
	}
}

func TestNetMgrSendMsg2All(t *testing.T) {
	mgr := NewNetMgr()
	s1 := &stubMsgQue{sessId: 1}
	s2 := &stubMsgQue{sessId: 2}
	mgr.sessAgent["logic"] = map[int32]IMsgQue{
		1: s1,
		2: s2,
	}

	mgr.SendMsg2All("logic", msg.NewMsg(pb.MSG_ID(1), []byte("a")), nil)
	drainTasks(mgr)

	if len(s1.sent) != 1 || len(s2.sent) != 1 {
		t.Fatalf("expected both sessions to receive message")
	}
}

func TestNetMgrSendMsg2One(t *testing.T) {
	mgr := NewNetMgr()
	s1 := &stubMsgQue{sessId: 1}
	s2 := &stubMsgQue{sessId: 2}
	mgr.sessAgent["logic"] = map[int32]IMsgQue{
		1: s1,
		2: s2,
	}

	mgr.SendMsg2One("logic", msg.NewMsg(pb.MSG_ID(1), []byte("a")), nil)
	drainTasks(mgr)

	got := len(s1.sent) + len(s2.sent)
	if got != 1 {
		t.Fatalf("expected one session to receive message, got %d", got)
	}
}

func TestNetMgrSendMsg2OneSendFail(t *testing.T) {
	mgr := NewNetMgr()
	s1 := &stubMsgQue{sessId: 1, hasRet: true, sendRet: false}
	mgr.sessAgent["logic"] = map[int32]IMsgQue{
		1: s1,
	}
	called := make(chan struct{}, 1)

	mgr.SendMsg2One("logic", msg.NewMsg(pb.MSG_ID(1), []byte("a")), func() {
		called <- struct{}{}
	})
	drainTasks(mgr)

	select {
	case <-called:
	default:
		t.Fatalf("fail callback should be called when Send returns false")
	}
}

func TestNetMgrSendMsg2OneAddTaskFail(t *testing.T) {
	mgr := &NetMgr{
		taskChan: make(chan func(), 1),
		stopCh:   make(chan struct{}),
	}
	mgr.taskChan <- func() {}
	called := make(chan struct{}, 1)

	mgr.SendMsg2One("logic", msg.NewMsg(pb.MSG_ID(1), []byte("a")), func() {
		called <- struct{}{}
	})

	select {
	case <-called:
	default:
		t.Fatalf("fail callback should be called when addTask fails")
	}
}

func TestNetMgrSendMsg2Fix(t *testing.T) {
	mgr := NewNetMgr()
	s1 := &stubMsgQue{sessId: 1}
	mgr.sessAgent["logic"] = map[int32]IMsgQue{
		1: s1,
	}

	mgr.SendMsg2Fix("logic", 1, msg.NewMsg(pb.MSG_ID(1), []byte("a")), nil)
	drainTasks(mgr)

	if len(s1.sent) != 1 {
		t.Fatalf("expected fixed session to receive message")
	}
}

func TestNetMgrSendMsg2FixSendFail(t *testing.T) {
	mgr := NewNetMgr()
	s1 := &stubMsgQue{sessId: 1, hasRet: true, sendRet: false}
	mgr.sessAgent["logic"] = map[int32]IMsgQue{
		1: s1,
	}
	called := make(chan struct{}, 1)

	mgr.SendMsg2Fix("logic", 1, msg.NewMsg(pb.MSG_ID(1), []byte("a")), func() {
		called <- struct{}{}
	})
	drainTasks(mgr)

	select {
	case <-called:
	default:
		t.Fatalf("fail callback should be called when fixed Send returns false")
	}
}

func TestNetMgrSendMsg2FixNoConn(t *testing.T) {
	mgr := NewNetMgr()
	called := make(chan struct{}, 1)

	mgr.SendMsg2Fix("logic", 1, msg.NewMsg(pb.MSG_ID(1), []byte("a")), func() {
		called <- struct{}{}
	})
	drainTasks(mgr)

	select {
	case <-called:
	default:
		t.Fatalf("fail callback not called")
	}
}

func TestNetMgrSendMsg2Sess(t *testing.T) {
	mgr := NewNetMgr()
	s1 := &stubMsgQue{sessId: 1}
	mgr.sessMap[1] = s1

	mgr.SendMsg2Sess(1, msg.NewMsg(pb.MSG_ID(1), []byte("a")), nil)
	drainTasks(mgr)

	if len(s1.sent) != 1 {
		t.Fatalf("expected session to receive message")
	}
}

func TestNetMgrSendMsg2SessNoConn(t *testing.T) {
	mgr := NewNetMgr()
	called := make(chan struct{}, 1)

	mgr.SendMsg2Sess(1, msg.NewMsg(pb.MSG_ID(1), []byte("a")), func() {
		called <- struct{}{}
	})
	drainTasks(mgr)

	select {
	case <-called:
	default:
		t.Fatalf("fail callback not called")
	}
}

func TestNetMgrSendMsg2AllUserNoPlayerNoFail(t *testing.T) {
	mgr := NewNetMgr()
	mgr.sessMap[1] = &stubMsgQue{sessId: 1}
	called := make(chan struct{}, 1)

	mgr.SendMsg2AllUser(msg.NewMsg(pb.MSG_ID(1), []byte("a")), func() {
		called <- struct{}{}
	})
	drainTasks(mgr)

	select {
	case <-called:
		t.Fatalf("fail callback should not be called when no player connection")
	default:
	}
}

func TestNetMgrSendMsg2AllUserAllSendFailNoFailCallback(t *testing.T) {
	mgr := NewNetMgr()
	agt := &ConnAgt{}
	agt.AddCltUser(10001)
	mgr.sessMap[1] = &stubMsgQue{
		sessId:  1,
		agt:     agt,
		hasRet:  true,
		sendRet: false,
	}
	called := make(chan struct{}, 1)

	mgr.SendMsg2AllUser(msg.NewMsg(pb.MSG_ID(1), []byte("a")), func() {
		called <- struct{}{}
	})
	drainTasks(mgr)

	select {
	case <-called:
		t.Fatalf("fail callback should not be called when all player sends fail")
	default:
	}
}

func TestNetMgrSendMsg2AllUserAddTaskFail(t *testing.T) {
	mgr := &NetMgr{
		taskChan: make(chan func(), 1),
		stopCh:   make(chan struct{}),
	}
	mgr.taskChan <- func() {}
	called := make(chan struct{}, 1)

	mgr.SendMsg2AllUser(msg.NewMsg(pb.MSG_ID(1), []byte("a")), func() {
		called <- struct{}{}
	})

	select {
	case <-called:
	default:
		t.Fatalf("fail callback should be called when addTask fails")
	}
}

func TestNetMgrStartListenAccept(t *testing.T) {
	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	opt := options.NewMsgQueOptions()
	opt.SetListenParams(options.NewListenParams("127.0.0.1:0"))

	if err := mgr.StartListen(opt, handler); err != nil {
		t.Fatalf("start listen failed: %v", err)
	}
	mqListen := mgr.listenMap[opt.ListenParams.ListenAddr].(*tcpMsgQue)
	addr := mqListen.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	waitForConn(t, handler.newCh, "accept")

	if mgr.GetAcceptSessNum() != 1 {
		t.Fatalf("accept session count mismatch")
	}
}

func TestNetMgrStartConnect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	addr := ln.Addr().String()
	opt := options.NewMsgQueOptions()
	opt.SetConnectParams(options.NewConnectParams(addr, "logic", 1))

	if err := mgr.StartConnect(opt, handler); err != nil {
		t.Fatalf("start connect failed: %v", err)
	}

	conn, err := ln.Accept()
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}
	defer conn.Close()

	waitForConn(t, handler.connectCh, "connect")

	if mgr.GetSessNum() == 0 {
		t.Fatalf("connect session not registered")
	}
}

func TestNetMgrCloseDuringReadTriggersStop(t *testing.T) {
	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	c1, c2 := net.Pipe()
	defer c1.Close()

	mq := setupMsgQueWithConn(t, mgr, handler, c1)

	_ = c2.Close()
	stopped := waitForConn(t, handler.stopCh, "stop on read close")
	if stopped.SessId() != mq.SessId() {
		t.Fatalf("stop event session mismatch")
	}
}

func TestTcpMsgQueStopUnblocksBlockingRead(t *testing.T) {
	opt := options.NewMsgQueOptions()
	opt.Timeout = 0
	opt.WriteChanSize = 1

	conn := newBlockingReadConn()
	mq := newTcpConnect(nil, opt)
	mq.conn = conn
	mq.Start()
	defer conn.Close()

	select {
	case <-conn.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("read did not start")
	}

	done := make(chan struct{})
	go func() {
		mq.stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("stop blocked on read")
	}
}

func TestNetMgrCloseDuringWriteTriggersStop(t *testing.T) {
	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	conn := newWriteReasonConn()
	mq := setupMsgQueWithConn(t, mgr, handler, conn)

	if !mq.Send(msg.NewMsg(pb.MSG_ID(1), []byte("data"))) {
		t.Fatalf("send failed")
	}

	select {
	case <-conn.wroteCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("write not started")
	}
	_ = conn.Close()

	stopped := waitForConn(t, handler.stopCh, "stop on write close")
	if stopped.SessId() != mq.SessId() {
		t.Fatalf("stop event session mismatch")
	}
}

func TestNetMgrActiveCloseTriggersStop(t *testing.T) {
	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	mq := setupMsgQueWithConn(t, mgr, handler, c1)
	mgr.RemoveSession(mq.SessId())

	stopped := waitForConn(t, handler.stopCh, "stop on remove session")
	if stopped.SessId() != mq.SessId() {
		t.Fatalf("stop event session mismatch")
	}
}

func TestNetMgrDisconnectEventAccept(t *testing.T) {
	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	opt := options.NewMsgQueOptions()
	opt.SetListenParams(options.NewListenParams("127.0.0.1:0"))

	if err := mgr.StartListen(opt, handler); err != nil {
		t.Fatalf("start listen failed: %v", err)
	}
	mqListen := mgr.listenMap[opt.ListenParams.ListenAddr].(*tcpMsgQue)
	addr := mqListen.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	waitForConn(t, handler.newCh, "accept")
	waitForCondition(t, "accept session count", func() bool {
		return mgr.GetAcceptSessNum() == 1
	})

	_ = conn.Close()
	waitForConn(t, handler.stopCh, "disconnect stop")
	waitForCondition(t, "accept session removed", func() bool {
		return mgr.GetAcceptSessNum() == 0
	})
}

func TestNetMgrDisconnectEventLogsTcpReason(t *testing.T) {
	readLog := captureDefaultLog(t)

	mgr := NewNetMgr()
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	c1, c2 := net.Pipe()
	defer c1.Close()

	mq := setupMsgQueWithConn(t, mgr, handler, c1)
	_ = c2.Close()

	stopped := waitForConn(t, handler.stopCh, "disconnect stop")
	if stopped.SessId() != mq.SessId() {
		t.Fatalf("stop event session mismatch")
	}
	waitForCondition(t, "session removed", func() bool {
		return mgr.GetSessNum() == 0
	})

	logOutput := readLog()
	if !strings.Contains(logOutput, "conn disconnect [") ||
		!strings.Contains(logOutput, "connType=conn") ||
		!strings.Contains(logOutput, "cause=read: EOF") ||
		!strings.Contains(logOutput, "action=target unavailable") {
		t.Fatalf("disconnect log missing tcp reason, logs: %s", logOutput)
	}
}

func TestNetMgrDisconnectCauseKeepsFirstTcpReason(t *testing.T) {
	mgr := NewNetMgr()
	opt := options.NewMsgQueOptions()
	mq := newTcpConnect(nil, opt)
	mq.recordDisconnectErr("write", io.ErrClosedPipe)
	mq.recordDisconnectErr("read", io.EOF)

	reason, action := mgr.disconnectReason(mq, true)
	if reason != "write: io: read/write on closed pipe" {
		t.Fatalf("unexpected disconnect cause: %s", reason)
	}
	if action != "reconnect scheduled" {
		t.Fatalf("unexpected disconnect action: %s", action)
	}
}

func TestNetMgrAcceptSessNumDisconnectScenarios(t *testing.T) {
	mgr := NewNetMgr()
	opt := options.NewMsgQueOptions()

	s1 := &stubMsgQue{sessId: 1, connTyp: ConnTypeAccept, agt: newConnAgt(), opt: opt}
	s2 := &stubMsgQue{sessId: 2, connTyp: ConnTypeAccept, agt: newConnAgt(), opt: opt}
	s3 := &stubMsgQue{sessId: 3, connTyp: ConnTypeAccept, agt: newConnAgt(), opt: opt}

	mgr.addSess(s1)
	mgr.addSess(s2)
	mgr.addSess(s3)

	if mgr.GetAcceptSessNum() != 3 {
		t.Fatalf("accept session count mismatch: %d", mgr.GetAcceptSessNum())
	}

	mgr.sessOverEvt(s1)
	drainTasks(mgr)
	if mgr.GetAcceptSessNum() != 2 {
		t.Fatalf("accept session count mismatch after disconnect: %d", mgr.GetAcceptSessNum())
	}
	waitForCondition(t, "session stopped after disconnect", func() bool { return s1.stopped.Load() })

	mgr.RemoveSession(s2.sessId)
	drainTasks(mgr)
	if mgr.GetAcceptSessNum() != 1 {
		t.Fatalf("accept session count mismatch after remove: %d", mgr.GetAcceptSessNum())
	}
	waitForCondition(t, "session stopped after remove", func() bool { return s2.stopped.Load() })

	s3.agt.AddCltUser(100)
	mgr.KickSession(s3.sessId, 100)
	drainTasks(mgr)
	if mgr.GetAcceptSessNum() != 0 {
		t.Fatalf("accept session count mismatch after kick: %d", mgr.GetAcceptSessNum())
	}
	waitForCondition(t, "session stopped after kick", func() bool { return s3.stopped.Load() })
}

func TestNetMgrReconnectAfterDisconnect(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	acceptCh := make(chan net.Conn, 4)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			acceptCh <- conn
		}
	}()

	mgr := NewNetMgr()
	mgr.RegisterCanReconnect(func(params *options.ConnectParams) bool { return true })
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	opt := options.NewMsgQueOptions()
	opt.SetConnectParams(options.NewConnectParams(ln.Addr().String(), "logic", 1))

	if err := mgr.StartConnect(opt, handler); err != nil {
		t.Fatalf("start connect failed: %v", err)
	}

	firstConn := waitForConn(t, handler.connectCh, "connect-1")
	conn1 := <-acceptCh

	_ = conn1.Close()
	waitForConn(t, handler.stopCh, "disconnect stop")

	conn2 := <-acceptCh
	defer conn2.Close()

	secondConn := waitForConn(t, handler.connectCh, "connect-2")
	if firstConn.SessId() == secondConn.SessId() {
		t.Fatalf("reconnect should use a new session id")
	}
}

func TestNetMgrNoReconnectWhenDisabled(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	acceptCh := make(chan net.Conn, 2)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			acceptCh <- conn
		}
	}()

	mgr := NewNetMgr()
	mgr.RegisterCanReconnect(func(params *options.ConnectParams) bool { return false })
	mgr.Start()
	defer mgr.Stop()

	handler := newRecordHandler()
	opt := options.NewMsgQueOptions()
	opt.SetConnectParams(options.NewConnectParams(ln.Addr().String(), "logic", 1))

	if err := mgr.StartConnect(opt, handler); err != nil {
		t.Fatalf("start connect failed: %v", err)
	}

	conn := <-acceptCh
	waitForConn(t, handler.connectCh, "connect-1")

	_ = conn.Close()
	waitForConn(t, handler.stopCh, "disconnect stop")

	select {
	case extra := <-acceptCh:
		_ = extra.Close()
		t.Fatalf("unexpected reconnect when disabled")
	case <-time.After(200 * time.Millisecond):
	}
}

func BenchmarkWriteDelayCtrlNextDelay(b *testing.B) {
	b.Run("HighBatch", func(b *testing.B) {
		ctrl := newTcpWriteDelayCtrl(8, WriteChanLowWatermark)
		var d time.Duration
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d = ctrl.nextDelay(WriteChanLowWatermark+1, true)
		}
		benchDelay = d
	})

	b.Run("LowBatch", func(b *testing.B) {
		ctrl := newTcpWriteDelayCtrl(8, WriteChanLowWatermark)
		var d time.Duration
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d = ctrl.nextDelay(WriteChanLowWatermark, true)
		}
		benchDelay = d
	})
}
