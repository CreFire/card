package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"game/deps/kit"
	"game/deps/xlog"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EventType 定义了 Watcher 事件的类型
type EventType string

const (
	// EventTypePut 表示一个 key 被创建或更新
	EventTypePut EventType = "PUT"
	// EventTypeDelete 表示一个 key 被删除
	EventTypeDelete EventType = "DELETE"
	// EventTypeSnapshot 表示一次全量数据快照
	EventTypeSnapshot EventType = "SNAPSHOT"
)

// EtcdWatchEvent 代表一个具体的 watch 事件
type EtcdWatchEvent struct {
	Type  EventType
	Key   string
	Value []byte
	Inst  *ServiceInstance
}

// WatchResponse 是发送给业务层的 watch 响应
type WatchResponse struct {
	Events []*EtcdWatchEvent
	Err    error
}

// ClientWatcher 封装了 etcd 的 watch 机制，提供了自动重连和快照同步功能
type ClientWatcher struct {
	client    *clientv3.Client
	key       string
	opts      []clientv3.OpOption
	ctx       context.Context
	cancel    context.CancelFunc
	eventChan chan *WatchResponse
	wg        sync.WaitGroup
}

// NewClientWatcher 创建一个新的 ClientWatcher 实例
// 它会立即启动一个后台 goroutine 来监听指定的 key
func NewClientWatcher(client *clientv3.Client, key string, pCtx context.Context, opts ...clientv3.OpOption) (*ClientWatcher, error) {
	if client == nil {
		return nil, errors.New("etcd client is nil")
	}

	// 使用父 context 创建一个可取消的 context，用于控制 watcher 的生命周期
	ctx, cancel := context.WithCancel(pCtx)

	w := &ClientWatcher{
		client:    client,
		key:       key,
		opts:      opts,
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan *WatchResponse, 256), // 使用带缓冲的 channel
	}

	w.wg.Add(1)
	go w.watchLoop()

	return w, nil
}

// EventChan 返回一个只读的 channel，业务层可以从中接收 WatchResponse
func (w *ClientWatcher) EventChan() <-chan *WatchResponse {
	return w.eventChan
}

// Close 停止 watcher 并关闭事件 channel
func (w *ClientWatcher) Close() {
	w.cancel()
	w.wg.Wait()
	close(w.eventChan)
}

// watchLoop 是 watcher 的主循环，负责处理连接和重连
func (w *ClientWatcher) watchLoop() {
	defer kit.Exception(nil)
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			// Watcher 被关闭
			return
		default:
			// 启动或重启 getAndWatch
			err := w.getAndWatch()
			if err != nil {
				// 如果是 context Canceled 或 DeadlineExceeded，说明是主动关闭，直接退出
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}

				// 其他错误，记录日志并发送到事件 channel
				xlog.Errorf("[ClientWatcher] Watch on key '%s' failed, will retry after 3 seconds. Error: %v", w.key, err)
				sendWatchResponse(w.ctx, w.eventChan, &WatchResponse{Err: err})

				// 等待一段时间后重试，避免频繁失败导致 CPU 空转
				select {
				case <-time.After(3 * time.Second):
				case <-w.ctx.Done():
					return
				}
			}
		}
	}
}

// getAndWatch 先获取一次全量数据（快照），然后从该版本开始监听增量变化
func (w *ClientWatcher) getAndWatch() error {
	// 1. 获取当前全量数据作为快照
	resp, err := w.client.Get(w.ctx, w.key, w.opts...)
	if err != nil {
		return fmt.Errorf("failed to get initial snapshot for key '%s': %w", w.key, err)
	}

	// 发送快照事件
	snapshotEvents := make([]*EtcdWatchEvent, len(resp.Kvs))
	for i, kv := range resp.Kvs {
		snapshotEvents[i] = &EtcdWatchEvent{
			Type:  EventTypeSnapshot,
			Key:   string(kv.Key),
			Value: kv.Value,
		}
		inst := &ServiceInstance{}
		err := json.Unmarshal(kv.Value, inst)
		if err != nil {
			xlog.Errorf("failed to unmarshal to instance key: %s value: %s  err: %v", kv.Key, kv.Value, err)
		} else {
			snapshotEvents[i].Inst = inst
		}
		xlog.Infof("[ClientWatcher] GET key: %s, value: %s", kv.Key, kv.Value)
	}
	sendWatchResponse(w.ctx, w.eventChan, &WatchResponse{Events: snapshotEvents})

	// 2. 从获取快照后的版本开始 Watch
	watchRevision := resp.Header.Revision + 1
	watchChan := w.client.Watch(w.ctx, w.key, clientv3.WithRev(watchRevision), clientv3.WithPrefix())

	xlog.Infof("[ClientWatcher] Watching key '%s' from revision %d, data", w.key, watchRevision)

	// 3. 循环处理 watch 事件
	for {
		select {
		case <-w.ctx.Done():
			return w.ctx.Err()
		case watchResp, ok := <-watchChan:
			if !ok {
				return errors.New("watch channel closed by etcd server")
			}
			if err := watchResp.Err(); err != nil {
				return fmt.Errorf("watch response error: %w", err)
			}

			if len(watchResp.Events) == 0 {
				continue // 心跳或无事件，忽略
			}

			events := make([]*EtcdWatchEvent, 0, len(watchResp.Events))
			for _, ev := range watchResp.Events {
				var eventType EventType
				var value []byte
				inst := &ServiceInstance{}
				switch ev.Type {
				case clientv3.EventTypePut:
					eventType = EventTypePut
					value = ev.Kv.Value // 对于 PUT 事件，Value 存在
					if err := json.Unmarshal(value, inst); err != nil {
						xlog.Errorf("failed to unmarshal instance on PUT, key: %s, value: %s, err: %v", ev.Kv.Key, ev.Kv.Value, err)
					}
					xlog.Infof("[ClientWatcher] PUT EVENT key: %s, value: %s", ev.Kv.Key, value)
				case clientv3.EventTypeDelete:
					eventType = EventTypeDelete
					value = nil
					inst = nil
					xlog.Infof("[ClientWatcher] DEL EVENT key: %s", ev.Kv.Key)
				default:
					continue // 忽略未知事件类型
				}

				events = append(events, &EtcdWatchEvent{
					Type:  eventType,
					Key:   string(ev.Kv.Key),
					Value: value,
					Inst:  inst,
				})
			}

			if len(events) > 0 {
				sendWatchResponse(w.ctx, w.eventChan, &WatchResponse{Events: events})
			}
		}
	}
}

// MultiWatcher 可同时监听多个 key 并将事件聚合到一个 channel 中
type MultiWatcher struct {
	client    *clientv3.Client
	keys      []string
	opts      []clientv3.OpOption
	ctx       context.Context
	cancel    context.CancelFunc
	eventChan chan *WatchResponse
	watchers  []*ClientWatcher
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// NewMultiWatcher 创建一个新的 MultiWatcher 实例
// 为每个 key 启动一个 ClientWatcher，并聚合所有事件到一个 channel 中
func NewMultiWatcher(client *clientv3.Client, keys []string, pCtx context.Context, opts ...clientv3.OpOption) (*MultiWatcher, error) {
	if client == nil {
		return nil, errors.New("etcd client is nil")
	}
	if len(keys) == 0 {
		return nil, errors.New("keys cannot be empty")
	}

	ctx, cancel := context.WithCancel(pCtx)

	mw := &MultiWatcher{
		client:    client,
		keys:      keys,
		opts:      opts,
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan *WatchResponse, 128), // Use a buffered channel
	}

	for _, key := range keys {
		watcher, err := NewClientWatcher(client, key, ctx, opts...)
		if err != nil {
			// 如果创建过程中出错，则清理之前创建的 watcher
			for _, w := range mw.watchers {
				w.Close()
			}
			cancel() // cancel the context for this multi-watcher
			return nil, fmt.Errorf("failed to create watcher for key '%s': %w", key, err)
		}
		mw.watchers = append(mw.watchers, watcher)

		// 启动 goroutine 转发该 watcher 的事件
		mw.wg.Add(1)
		go func(w *ClientWatcher) {
			defer kit.Exception(nil)
			defer mw.wg.Done()
			for {
				select {
				case resp, ok := <-w.EventChan():
					if !ok {
						return
					}
					sendWatchResponse(mw.ctx, mw.eventChan, resp)
				case <-mw.ctx.Done():
					return
				}
			}
		}(watcher)
	}

	// 父 context 被取消时，自动执行关闭，确保 EventChan 可被消费方正确退出。
	go func() {
		<-ctx.Done()
		mw.Close()
	}()

	return mw, nil
}

// EventChan 返回一个只读的 channel，业务层可以从中接收 WatchResponse
func (mw *MultiWatcher) EventChan() <-chan *WatchResponse {
	return mw.eventChan
}

// Close 关闭所有底层的 watcher 并关闭事件 channel
func (mw *MultiWatcher) Close() {
	mw.closeOnce.Do(func() {
		mw.cancel() // 触发所有底层 watcher 及转发 goroutine 的退出
		mw.wg.Wait()
		close(mw.eventChan)
		xlog.Infof("[MultiWatcher] Closed watcher for keys: %v", mw.keys)
	})
}

// sendWatchResponse avoids blocking and logs when the channel is full.
func sendWatchResponse(ctx context.Context, ch chan *WatchResponse, resp *WatchResponse) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- resp:
		return true
	default:
		xlog.Errorf("[Watcher] event channel full, dropping response: %+v", resp)
	}
	return false
}
