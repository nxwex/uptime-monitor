package storage

import (
	"errors"
	"sync"
	"time"

	"github.com/nxwex/uptime-monitor/internal/models"
)

var (
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
)

type Monitors struct {
	mu         sync.RWMutex
	monCounter int
	items      map[int]*models.Monitor
}

func NewMonitors() *Monitors {
	return &Monitors{
		monCounter: 0,
		items:      make(map[int]*models.Monitor),
	}
}

func (m *Monitors) Add(url string, interval time.Duration) (*models.Monitor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mon := range m.items {
		if url == mon.Url {
			return mon, ErrAlreadyExists
		}
	}

	m.monCounter++
	newMon := models.NewMonitor(m.monCounter, url, interval)
	m.items[m.monCounter] = newMon

	return newMon, nil
}

func (m *Monitors) GetAll() []*models.Monitor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]*models.Monitor, 0, len(m.items))
	for _, v := range m.items {
		res = append(res, v)
	}

	return res
}

func (m *Monitors) Get(id int) (*models.Monitor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if mon, ok := m.items[id]; ok {
		return mon, nil
	}

	return nil, ErrNotFound
}

func (m *Monitors) Delete(id int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.items[id]; ok {
		delete(m.items, id)
		return true
	}

	return false
}

func (m *Monitors) UpdateStatus(id int, status string, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mon, ok := m.items[id]
	if !ok {
		return ErrNotFound
	}

	mon.Status = status
	mon.LastCheck = time.Now()
	mon.ResponseTimeMs = duration

	newResult := models.CheckResult{
		Status:       status,
		ResponseTime: duration,
		CheckedAt:    time.Now(),
	}

	mon.History = append([]models.CheckResult{newResult}, mon.History...)
	if len(mon.History) > 3 {
		mon.History = mon.History[:3]
	}

	return nil
}
