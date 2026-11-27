package server

import (
	"encoding/json"
	"fmt"
	nethttp "net/http"
	"strings"
	"time"

	"hoptrace/internal/biz"
)

// sseHandler 提供 GET /api/v1/events/stream 实时事件流(Server-Sent Events)。
// 浏览器原生 EventSource 不支持自定义 Authorization 头, 开启鉴权时允许通过 ?token=<jwt> 认证。
func sseHandler(bus *biz.EventBus, auth *biz.AuthUsecase) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if auth != nil && auth.Enabled() {
			tok := r.URL.Query().Get("token")
			if tok == "" {
				if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
					tok = strings.TrimPrefix(h, "Bearer ")
				}
			}
			if _, err := auth.ParseToken(tok); err != nil {
				nethttp.Error(w, "unauthorized", nethttp.StatusUnauthorized)
				return
			}
		}

		flusher, ok := w.(nethttp.Flusher)
		if !ok {
			nethttp.Error(w, "streaming unsupported", nethttp.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // 防止 Nginx 缓冲 SSE

		ch, unsub := bus.Subscribe()
		defer unsub()

		// 立即发送连接确认帧
		_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"ok\":true}\n\n")
		flusher.Flush()

		heartbeat := time.NewTicker(25 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-heartbeat.C:
				_, _ = fmt.Fprintf(w, ": ping\n\n")
				flusher.Flush()
			case ev, open := <-ch:
				if !open {
					return
				}
				b, err := json.Marshal(ev)
				if err != nil {
					continue
				}
				_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, b)
				flusher.Flush()
			}
		}
	})
}
