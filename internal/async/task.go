package async

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"

	"github.com/dataflowagenthub/hub/internal/ssebus"
)

// 默认任务超时时间
const defaultTaskTimeout = 120 * time.Second

var taskTimeoutCounter = promauto.NewCounter(prometheus.CounterOpts{
	Name: "hub_task_timeout_total",
	Help: "Total number of async tasks that timed out",
})

type Task struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id"`
	TaskType  string         `json:"task_type"`
	Payload   map[string]any `json:"payload"`
}

// Client 封装了异步任务队列的客户端。
// 支持 NATS 消息发布和任务超时检测。
type Client struct {
	DB      *pgxpool.Pool
	NATS    *nats.Conn
	Log     *zap.Logger
	Timeout time.Duration // 任务超时时间（默认 120s），0 表示不检测超时
	Bus     ssebus.Bus    // SSE 总线，用于超时通知（可为 nil）
}

func NewClient(db *pgxpool.Pool, nc *nats.Conn, log *zap.Logger) *Client {
	return &Client{
		DB:      db,
		NATS:    nc,
		Log:     log,
		Timeout: defaultTaskTimeout,
		Bus:     nil,
	}
}

// EnqueueTask 将任务插入数据库，发布 NATS 消息，并启动超时检测。
func (c *Client) EnqueueTask(ctx context.Context, wsID, sessionID, runID, taskType string, payload map[string]any) (string, error) {
	var taskID string
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal task payload: %w", err)
	}

	err = c.DB.QueryRow(ctx, `
		INSERT INTO async_tasks (workspace_id, session_id, run_id, task_type, payload, status)
		VALUES ($1::uuid, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, $4, $5, 'queued')
		RETURNING id::text`,
		wsID, sessionID, runID, taskType, payloadJSON).Scan(&taskID)
	if err != nil {
		return "", err
	}

	task := Task{
		ID:        taskID,
		SessionID: sessionID,
		RunID:     runID,
		TaskType:  taskType,
		Payload:   payload,
	}
	msg, marshalErr := json.Marshal(task)
	if marshalErr != nil {
		c.Log.Warn("marshal task for nats", zap.Error(marshalErr))
		msg = []byte("{}")
	}

	if c.NATS != nil {
		if err := c.NATS.Publish("hub.tasks."+taskType, msg); err != nil {
			c.Log.Error("publish to nats failed", zap.Error(err))
			// 即使 NATS 发布失败，也返回任务 ID，可通过轮询获取
		}
	}

	// 启动超时检测 goroutine
	if c.Timeout > 0 {
		c.startTimeoutWatcher(taskID, sessionID, c.Timeout)
	}

	return taskID, nil
}

// startTimeoutWatcher 启动一个后台 goroutine，在超时后检查任务状态。
// 如果任务仍为 queued/running，则更新为 timeout 并通过 SSE 通知。
func (c *Client) startTimeoutWatcher(taskID, sessionID string, timeout time.Duration) {
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		select {
		case <-timer.C:
			// 超时后检查任务状态
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var status string
			err := c.DB.QueryRow(ctx,
				`SELECT status FROM async_tasks WHERE id = $1::uuid`, taskID,
			).Scan(&status)

			if err != nil {
				c.Log.Warn("timeout checker: failed to query task",
					zap.String("task_id", taskID),
					zap.Error(err),
				)
				return
			}

			// 只有仍然处于 queued 或 running 状态的任务才标记超时
			if status != "queued" && status != "running" {
				return
			}

			_, err = c.DB.Exec(ctx,
				`UPDATE async_tasks SET status = 'timeout', updated_at = now(), error_message = 'task timed out after ' || $2::text WHERE id = $1::uuid AND status IN ('queued', 'running')`,
				taskID, timeout.String(),
			)
			if err != nil {
				c.Log.Error("timeout checker: failed to update task",
					zap.String("task_id", taskID),
					zap.Error(err),
				)
				return
			}

			// 记录 Prometheus 指标
			taskTimeoutCounter.Inc()

			c.Log.Warn("task timed out",
				zap.String("task_id", taskID),
				zap.String("session_id", sessionID),
				zap.Duration("timeout", timeout),
			)

			// 通过 SSE 通知前端
			if c.Bus != nil && sessionID != "" {
				c.Bus.Publish(sessionID, ssebus.Event{
					Type: "task_timeout",
					Data: map[string]string{
						"task_id": taskID,
						"message": "task timed out after " + timeout.String(),
					},
				})
			}
		}
	}()
}

// SetTaskTimeout 设置任务超时时间。设为 0 可禁用超时检测。
func (c *Client) SetTaskTimeout(timeout time.Duration) {
	c.Timeout = timeout
}

// SetBus 设置 SSE 总线实例，用于超时通知。
func (c *Client) SetBus(bus ssebus.Bus) {
	c.Bus = bus
}

// StartReaper 启动后台 goroutine，定期清理过期任务。
func (c *Client) StartReaper(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				res, err := c.DB.Exec(ctx, `
					UPDATE async_tasks
					SET status = 'expired', updated_at = now()
					WHERE status IN ('queued', 'running') AND expires_at < now()`)
				if err != nil {
					c.Log.Error("failed to reap expired tasks", zap.Error(err))
				} else if res.RowsAffected() > 0 {
					c.Log.Info("reaped expired tasks", zap.Int64("count", res.RowsAffected()))
				}
			}
		}
	}()
}
