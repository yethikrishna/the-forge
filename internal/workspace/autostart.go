// Package workspace provides auto-start and auto-sleep for environments.
// Workspaces start on first access and sleep after being idle, reducing
// resource consumption without losing state.
package workspace

import (
	"context"
	"sync"
	"time"
)

// AutoStartPolicy controls how an environment is started on demand.
type AutoStartPolicy string

const (
	AutoStartAlways AutoStartPolicy = "always"
	AutoStartOnDemand AutoStartPolicy = "on_demand"
	AutoStartNever    AutoStartPolicy = "never"
)

// SleepPolicy controls when an idle environment goes to sleep.
type SleepPolicy string

const (
	SleepAfter5m  SleepPolicy = "5m"
	SleepAfter15m SleepPolicy = "15m"
	SleepAfter30m SleepPolicy = "30m"
	SleepAfter1h  SleepPolicy = "1h"
	SleepNever    SleepPolicy = "never"
)

// AutoStartConfig defines auto-start and auto-sleep behavior.
type AutoStartConfig struct {
	EnvName      string          `json:"env_name"`
	StartPolicy  AutoStartPolicy `json:"start_policy"`
	SleepPolicy  SleepPolicy     `json:"sleep_policy"`
	MinUptime    time.Duration   `json:"min_uptime"`
	LastAccessed time.Time       `json:"last_accessed"`
	LastStarted  time.Time       `json:"last_started"`
	SleepCount   int             `json:"sleep_count"`
	WakeCount    int             `json:"wake_count"`
}

// AutoStartManager handles on-demand start and idle sleep.
type AutoStartManager struct {
	provisioner *Provisioner
	mu          sync.RWMutex
	configs     map[string]*AutoStartConfig
	stopCh      chan struct{}
}

// NewAutoStartManager creates an auto-start manager.
func NewAutoStartManager(provisioner *Provisioner) *AutoStartManager {
	m := &AutoStartManager{
		provisioner: provisioner,
		configs:     make(map[string]*AutoStartConfig),
		stopCh:      make(chan struct{}),
	}
	return m
}

// Configure sets auto-start/sleep policy for an environment.
func (m *AutoStartManager) Configure(envName string, startPolicy AutoStartPolicy, sleepPolicy SleepPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := &AutoStartConfig{
		EnvName:      envName,
		StartPolicy:  startPolicy,
		SleepPolicy:  sleepPolicy,
		MinUptime:    2 * time.Minute,
		LastAccessed: time.Now(),
	}
	if existing, ok := m.configs[envName]; ok {
		cfg.SleepCount = existing.SleepCount
		cfg.WakeCount = existing.WakeCount
		cfg.LastStarted = existing.LastStarted
	}
	m.configs[envName] = cfg
	return nil
}

// GetConfig retrieves auto-start config for an env.
func (m *AutoStartManager) GetConfig(envName string) (*AutoStartConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg, ok := m.configs[envName]
	if !ok {
		return nil, false
	}
	// Return copy
	cp := *cfg
	return &cp, true
}

// RecordAccess updates the last-accessed timestamp for an env.
func (m *AutoStartManager) RecordAccess(envName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg, ok := m.configs[envName]; ok {
		cfg.LastAccessed = time.Now()
	}
}

// WakeUp starts an environment on demand if it's stopped.
func (m *AutoStartManager) WakeUp(ctx context.Context, envName string) (*ProvisionedEnv, error) {
	m.mu.Lock()
	cfg, ok := m.configs[envName]
	if !ok {
		cfg = &AutoStartConfig{
			EnvName:      envName,
			StartPolicy:  AutoStartOnDemand,
			SleepPolicy:  SleepAfter15m,
			LastAccessed: time.Now(),
		}
		m.configs[envName] = cfg
	}
	cfg.LastAccessed = time.Now()
	cfg.LastStarted = time.Now()
	cfg.WakeCount++
	m.mu.Unlock()

	env, err := m.provisioner.Get(envName)
	if err != nil {
		return nil, err
	}

	if env.State == StateRunning {
		return env, nil
	}

	return m.provisioner.Start(ctx, envName)
}

// ShouldSleep returns whether an env should be put to sleep.
func (m *AutoStartManager) ShouldSleep(envName string) bool {
	m.mu.RLock()
	cfg, ok := m.configs[envName]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	if cfg.SleepPolicy == SleepNever {
		return false
	}

	env, err := m.provisioner.Get(envName)
	if err != nil || env.State != StateRunning {
		return false
	}

	// Don't sleep if just started (min uptime)
	if !cfg.LastStarted.IsZero() && time.Since(cfg.LastStarted) < cfg.MinUptime {
		return false
	}

	idleDuration := time.Since(cfg.LastAccessed)
	return idleDuration >= sleepPolicyDuration(cfg.SleepPolicy)
}

// SleepIfNeeded checks all managed envs and sleeps idle ones.
func (m *AutoStartManager) SleepIfNeeded(ctx context.Context) []string {
	var slept []string

	m.mu.RLock()
	envs := make([]string, 0, len(m.configs))
	for name := range m.configs {
		envs = append(envs, name)
	}
	m.mu.RUnlock()

	for _, name := range envs {
		if m.ShouldSleep(name) {
			if err := m.provisioner.Stop(ctx, name); err == nil {
				m.mu.Lock()
				if cfg, ok := m.configs[name]; ok {
					cfg.SleepCount++
				}
				m.mu.Unlock()
				slept = append(slept, name)
			}
		}
	}
	return slept
}

// StartBackgroundWatcher starts a goroutine that periodically checks idle envs.
func (m *AutoStartManager) StartBackgroundWatcher(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.SleepIfNeeded(ctx)
			case <-m.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopBackgroundWatcher stops the idle watcher.
func (m *AutoStartManager) StopBackgroundWatcher() {
	close(m.stopCh)
}

// sleepPolicyDuration converts a SleepPolicy to a time.Duration.
func sleepPolicyDuration(p SleepPolicy) time.Duration {
	switch p {
	case SleepAfter5m:
		return 5 * time.Minute
	case SleepAfter15m:
		return 15 * time.Minute
	case SleepAfter30m:
		return 30 * time.Minute
	case SleepAfter1h:
		return time.Hour
	default:
		return 0
	}
}
