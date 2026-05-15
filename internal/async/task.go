package async

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Task struct {
	ID        string         `json:"id"`
	SessionID string         `json:"session_id"`
	RunID     string         `json:"run_id"`
	TaskType  string         `json:"task_type"`
	Payload   map[string]any `json:"payload"`
}

type Client struct {
	DB   *pgxpool.Pool
	NATS *nats.Conn
	Log  *zap.Logger
}

func NewClient(db *pgxpool.Pool, nc *nats.Conn, log *zap.Logger) *Client {
	return &Client{
		DB:   db,
		NATS: nc,
		Log:  log,
	}
}

func (c *Client) EnqueueTask(ctx context.Context, wsID, sessionID, runID, taskType string, payload map[string]any) (string, error) {
	var taskID string
	payloadJSON, _ := json.Marshal(payload)

	err := c.DB.QueryRow(ctx, `
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
	msg, _ := json.Marshal(task)

	if c.NATS != nil {
		if err := c.NATS.Publish("hub.tasks."+taskType, msg); err != nil {
			c.Log.Error("publish to nats failed", zap.Error(err))
			// Even if NATS publish fails, we return the task ID, it could be picked up by polling
		}
	}

	return taskID, nil
}

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
