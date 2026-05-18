package ssebus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Event 是一个小型可 JSON 序列化的 SSE 负载
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Logger 用于 SSE 事件丢弃告警
type Logger interface {
	Info(string, ...zap.Field)
	Warn(string, ...zap.Field)
}

// Bus 是 SSE 事件总线的接口，支持内存和 Redis 两种实现。
type Bus interface {
	// Subscribe 订阅指定会话的事件流，返回只读 channel 和取消函数。
	Subscribe(sessionID string) (<-chan Event, func())
	// Publish 向指定会话发布事件。
	Publish(sessionID string, ev Event)
	// TotalDrops 返回总线丢弃的事件总数（仅 MemoryBus 实现有意义）。
	TotalDrops() int64
}

// --- MemoryBus ---

// MemoryBus 使用内存 map 存储订阅者的 SSE 总线实现。
type MemoryBus struct {
	mu         sync.Mutex
	subs       map[string][]chan Event
	totalDrops atomic.Int64
	log        Logger
}

// NewMemoryBus 创建一个新的 MemoryBus 实例。
func NewMemoryBus(log Logger) *MemoryBus {
	return &MemoryBus{
		subs: make(map[string][]chan Event),
		log:  log,
	}
}

func (b *MemoryBus) Subscribe(sessionID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.subs[sessionID] = append(b.subs[sessionID], ch)
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		list := b.subs[sessionID]
		out := list[:0]
		for _, c := range list {
			if c != ch {
				out = append(out, c)
			} else {
				close(c)
			}
		}
		if len(out) == 0 {
			delete(b.subs, sessionID)
		} else {
			b.subs[sessionID] = out
		}
	}

	return ch, cancel
}

func (b *MemoryBus) TotalDrops() int64 { return b.totalDrops.Load() }

func (b *MemoryBus) Publish(sessionID string, ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subs[sessionID] {
		select {
		case ch <- ev:
		default:
			drops := b.totalDrops.Add(1)
			if b.log != nil && drops%10 == 0 {
				b.log.Warn("sse event dropped",
					zap.String("session_id", sessionID),
					zap.Int64("total_drops", drops),
				)
			}
		}
	}
}

// --- RedisBus ---

// RedisBus 使用 Redis Pub/Sub 实现跨进程 SSE 事件总线。
type RedisBus struct {
	client *redis.Client
	log    Logger
}

// NewRedisBus 创建一个新的 RedisBus 实例。
func NewRedisBus(client *redis.Client, log Logger) *RedisBus {
	return &RedisBus{client: client, log: log}
}

// redisChannelName 返回 Redis Pub/Sub 频道名称。
func redisChannelName(sessionID string) string {
	return fmt.Sprintf("hub:sse:%s", sessionID)
}

func (b *RedisBus) Subscribe(sessionID string) (<-chan Event, func()) {
	channelName := redisChannelName(sessionID)
	ch := make(chan Event, 32)

	ctx, cancel := context.WithCancel(context.Background())

	pubsub := b.client.Subscribe(ctx, channelName)

	// 等待订阅确认
	_, err := pubsub.Receive(ctx)
	if err != nil {
		b.log.Warn("redis subscribe failed, returning closed channel",
			zap.String("channel", channelName),
			zap.Error(err),
		)
		cancel()
		close(ch)
		return ch, func() {}
	}

	// 后台 goroutine：将 Redis 消息解析为 Event 并转发到 ch
	go func() {
		defer pubsub.Close()

		redisCh := pubsub.Channel(
			redis.WithChannelSize(32),
			redis.WithChannelHealthCheckInterval(10*time.Second),
		)

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-redisCh:
				if !ok {
					return
				}
				var ev Event
				if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
					b.log.Warn("redis unmarshal event failed",
						zap.String("channel", channelName),
						zap.Error(err),
					)
					continue
				}
				select {
				case ch <- ev:
				default:
					b.log.Warn("sse event dropped (redis bus)",
						zap.String("session_id", sessionID),
						zap.String("type", ev.Type),
					)
				}
			}
		}
	}()

	// 取消函数：取消 context -> 退出 goroutine -> pubsub.Close()
	cancelFn := func() {
		cancel()
	}

	return ch, cancelFn
}

func (b *RedisBus) Publish(sessionID string, ev Event) {
	channelName := redisChannelName(sessionID)
	data, err := json.Marshal(ev)
	if err != nil {
		b.log.Warn("redis marshal event failed",
			zap.String("session_id", sessionID),
			zap.Error(err),
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := b.client.Publish(ctx, channelName, data).Err(); err != nil {
		b.log.Warn("redis publish failed",
			zap.String("session_id", sessionID),
			zap.Error(err),
		)
	}
}

// TotalDrops 对 RedisBus 始终返回 0（Redis 自身处理缓冲）。
func (b *RedisBus) TotalDrops() int64 { return 0 }

// --- 工厂函数 ---

// NewBus 根据 SSE_DRIVER 环境变量创建合适的总线实现。
// SSE_DRIVER=redis 且 Redis 连接正常时返回 RedisBus，否则返回 MemoryBus。
func NewBus(rdb *redis.Client, log Logger) Bus {
	driver := os.Getenv("SSE_DRIVER")
	if driver == "redis" && rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Warn("SSE_DRIVER=redis but redis unavailable, falling back to memory bus",
				zap.Error(err),
			)
			return NewMemoryBus(log)
		}
		log.Info("sse bus using redis driver")
		return NewRedisBus(rdb, log)
	}
	if log != nil {
		log.Info("sse bus using memory driver", zap.String("sse_driver", driver))
	}
	return NewMemoryBus(log)
}
