// Package mysqlmgr 管理数据集对应的 MySQL 数据库连接池。
// 每个数据集对应一个独立的 MySQL 数据库，通过 dataset_id 作为键管理连接池生命周期。
package mysqlmgr

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// MySQLConfig 存储平台级 MySQL 连接配置。
type MySQLConfig struct {
	Host     string
	Port     int
	RootUser string
	RootPass string
}

// Manager 管理所有数据集的 MySQL 连接池。
type Manager struct {
	mu    sync.RWMutex
	pools map[string]*sql.DB
	cfg   MySQLConfig
	log   *zap.Logger
}

// NewManager 创建 MySQL 连接池管理器。
func NewManager(cfg MySQLConfig, log *zap.Logger) *Manager {
	return &Manager{
		pools: make(map[string]*sql.DB),
		cfg:   cfg,
		log:   log,
	}
}

// RootDSN 返回 MySQL root 用户的 DSN，用于管理操作（建库、删库）。
func (m *Manager) RootDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/",
		m.cfg.RootUser, m.cfg.RootPass,
		m.cfg.Host, m.cfg.Port,
	)
}

// databaseDSN 返回指定数据库的 DSN。
func (m *Manager) databaseDSN(dbName string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		m.cfg.RootUser, m.cfg.RootPass,
		m.cfg.Host, m.cfg.Port, dbName,
	)
}

// Connect 建立并缓存指定数据库的连接池。
func (m *Manager) Connect(datasetID, mysqlDatabase string) (*sql.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool, ok := m.pools[datasetID]; ok {
		return pool, nil
	}

	dsn := m.databaseDSN(mysqlDatabase)
	pool, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysqlmgr: open %s: %w", mysqlDatabase, err)
	}
	pool.SetMaxOpenConns(10)
	pool.SetMaxIdleConns(5)
	pool.SetConnMaxLifetime(30 * time.Minute)

	if err := pool.Ping(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("mysqlmgr: ping %s: %w", mysqlDatabase, err)
	}

	m.pools[datasetID] = pool
	m.log.Info("mysqlmgr: connected to dataset",
		zap.String("dataset_id", datasetID),
		zap.String("mysql_database", mysqlDatabase),
	)
	return pool, nil
}

// GetPool 返回指定数据集的连接池。
func (m *Manager) GetPool(datasetID string) (*sql.DB, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pool, ok := m.pools[datasetID]
	return pool, ok
}

// Close 关闭指定数据集的连接池。
func (m *Manager) Close(datasetID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pool, ok := m.pools[datasetID]; ok {
		pool.Close()
		delete(m.pools, datasetID)
		m.log.Info("mysqlmgr: closed dataset connection",
			zap.String("dataset_id", datasetID),
		)
	}
}

// CloseAll 关闭所有数据集的连接池。
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, pool := range m.pools {
		pool.Close()
		delete(m.pools, id)
	}
	m.log.Info("mysqlmgr: closed all connections")
}
