package biz

import (
	"sync"
	"time"
)

// LiveEvent 后端实时推送给前端 SSE 客户端的通知事件。
type LiveEvent struct {
	Type    string `json:"type"`              // topology_updated / trap_received
	Source  string `json:"source,omitempty"`  // trap / poll / discovery / manual
	Device  string `json:"device,omitempty"`  // 设备名称或 IP
	Message string `json:"message,omitempty"` // 人类可读摘要
	At      int64  `json:"at"`
}

// EventBus 轻量进程内发布/订阅总线, 供 SSE 推送使用。
type EventBus struct {
	mu   sync.RWMutex
	subs map[chan LiveEvent]struct{}
}

func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[chan LiveEvent]struct{})}
}

// Subscribe 注册订阅者并返回事件通道与取消函数。
func (b *EventBus) Subscribe() (<-chan LiveEvent, func()) {
	ch := make(chan LiveEvent, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
}

// Publish 非阻塞广播事件给全部在线客户端(慢客户端自动丢弃旧消息)。
func (b *EventBus) Publish(ev LiveEvent) {
	if b == nil {
		return
	}
	if ev.At == 0 {
		ev.At = time.Now().Unix()
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}
