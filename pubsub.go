package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
	"github.com/redis/go-redis/v9"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	pubSubBatchEvent = "pubsub:batch"
	// 消息合并推送的间隔：高频频道下避免每条消息都跨进程通信
	pubSubFlushInterval = 120 * time.Millisecond
	pubSubMaxBatch      = 200
)

// PubSubMessage 推给前端的一条订阅消息
type PubSubMessage struct {
	ConnID  string `json:"conn_id"`
	Channel string `json:"channel"`
	Payload string `json:"payload"`
	Pattern string `json:"pattern,omitempty"`
	Size    int    `json:"size"`
	At      int64  `json:"at"`
}

// PubSubStatus 订阅状态
type PubSubStatus struct {
	Running  bool     `json:"running"`
	Channels []string `json:"channels"`
	Patterns []string `json:"patterns"`
	Count    int64    `json:"count"`
	Error    string   `json:"error,omitempty"`
}

type pubSubSession struct {
	connID   string
	channels []string
	patterns []string
	subs     []*redis.PubSub
	stop     chan struct{}
	wg       sync.WaitGroup

	mu      sync.Mutex
	buffer  []PubSubMessage
	count   int64
	lastErr string
}

func (s *pubSubSession) append(msg PubSubMessage) {
	s.mu.Lock()
	s.count++
	if len(s.buffer) < pubSubMaxBatch*4 {
		s.buffer = append(s.buffer, msg)
	}
	s.mu.Unlock()
}

func (s *pubSubSession) take() []PubSubMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buffer) == 0 {
		return nil
	}
	out := s.buffer
	s.buffer = nil
	return out
}

func (s *pubSubSession) snapshot() PubSubStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return PubSubStatus{
		Running:  true,
		Channels: append([]string{}, s.channels...),
		Patterns: append([]string{}, s.patterns...),
		Count:    s.count,
		Error:    s.lastErr,
	}
}

// StartPubSub 订阅若干频道 / 模式，消息按批（120ms）推给前端
func (a *App) StartPubSub(connID string, channels []string, patterns []string) config.OperationResult {
	channels = normalizePubSubTargets(channels)
	patterns = normalizePubSubTargets(patterns)
	if len(channels) == 0 && len(patterns) == 0 {
		return config.OperationResult{Success: false, Message: "请填写至少一个频道或模式"}
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return config.OperationResult{Success: false, Message: err.Error()}
	}

	a.stopPubSub(connID)

	sess := &pubSubSession{
		connID:   connID,
		channels: channels,
		patterns: patterns,
		stop:     make(chan struct{}),
	}
	pubsubCtx := a.pubSubContext()
	if len(channels) > 0 {
		sess.subs = append(sess.subs, client.Subscribe(pubsubCtx, channels...))
	}
	if len(patterns) > 0 {
		sess.subs = append(sess.subs, client.PSubscribe(pubsubCtx, patterns...))
	}

	for _, sub := range sess.subs {
		ch := sub.Channel()
		sess.wg.Add(1)
		go func(messages <-chan *redis.Message) {
			defer sess.wg.Done()
			for msg := range messages {
				sess.append(PubSubMessage{
					ConnID:  connID,
					Channel: msg.Channel,
					Payload: msg.Payload,
					Pattern: msg.Pattern,
					Size:    len(msg.Payload),
					At:      time.Now().UnixMilli(),
				})
			}
		}(ch)
	}

	// 定时把缓冲区里的消息合并成一次事件推给前端
	sess.wg.Add(1)
	go func() {
		defer sess.wg.Done()
		ticker := time.NewTicker(pubSubFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-sess.stop:
				a.flushPubSub(sess)
				return
			case <-ticker.C:
				a.flushPubSub(sess)
			}
		}
	}()

	a.pubsubMu.Lock()
	if a.pubsub == nil {
		a.pubsub = make(map[string]*pubSubSession)
	}
	a.pubsub[connID] = sess
	a.pubsubMu.Unlock()

	config.AppendDebugLog("[pubsub] started conn=%s channels=%v patterns=%v", connID, channels, patterns)
	return config.OperationResult{Success: true}
}

// pubSubContext 订阅用的 context（单测里 a.ctx 为空时退回 Background，避免 go-redis panic）
func (a *App) pubSubContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) flushPubSub(sess *pubSubSession) {
	batch := sess.take()
	if len(batch) == 0 || a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, pubSubBatchEvent, batch)
}

// PublishMessage 向频道发布一条消息，返回收到该消息的订阅者数量
func (a *App) PublishMessage(connID, channel, payload string) (int64, error) {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return 0, errors.New("请填写要发布的频道")
	}
	client, err := a.manager.GetClient(connID)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(a.pubSubContext(), 5*time.Second)
	defer cancel()
	return client.Publish(ctx, channel, payload).Result()
}

// StopPubSub 停止订阅
func (a *App) StopPubSub(connID string) config.OperationResult {
	a.stopPubSub(connID)
	return config.OperationResult{Success: true}
}

func (a *App) stopPubSub(connID string) {
	a.pubsubMu.Lock()
	sess := a.pubsub[connID]
	delete(a.pubsub, connID)
	a.pubsubMu.Unlock()
	if sess == nil {
		return
	}
	for _, sub := range sess.subs {
		_ = sub.Close()
	}
	close(sess.stop)
	sess.wg.Wait()
	a.flushPubSub(sess)
	config.AppendDebugLog("[pubsub] stopped conn=%s", connID)
}

// GetPubSubStatus 当前订阅状态（前端打开面板时同步用）
func (a *App) GetPubSubStatus(connID string) PubSubStatus {
	a.pubsubMu.Lock()
	sess := a.pubsub[connID]
	a.pubsubMu.Unlock()
	if sess == nil {
		return PubSubStatus{Running: false}
	}
	return sess.snapshot()
}

func (a *App) stopAllPubSub() {
	a.pubsubMu.Lock()
	ids := make([]string, 0, len(a.pubsub))
	for id := range a.pubsub {
		ids = append(ids, id)
	}
	a.pubsubMu.Unlock()
	for _, id := range ids {
		a.stopPubSub(id)
	}
}

func normalizePubSubTargets(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
