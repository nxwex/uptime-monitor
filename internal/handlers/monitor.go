package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nxwex/uptime-monitor/internal/models"
	"github.com/nxwex/uptime-monitor/internal/storage"
	"github.com/nxwex/uptime-monitor/internal/worker"
)

var startTime = time.Now()

type MonitorStore interface {
	Add(url string, interval time.Duration) (*models.Monitor, error)
	GetAll() []*models.Monitor
	Get(id int) (*models.Monitor, error)
	Delete(id int) bool
}

type MonitorStorage struct {
	Storage   MonitorStore
	Scheduler *worker.Scheduler
}

type userRequest struct {
	URL      string `json:"url"`
	Interval int    `json:"interval"`
}

type MonitorResponse struct {
	ID             int                  `json:"id"`
	Url            string               `json:"url"`
	Interval       int                  `json:"interval"`
	Status         string               `json:"status"`
	ResponseTimeMs int64                `json:"response_time_ms"`
	LastCheck      time.Time            `json:"last_check"`
	History        []models.CheckResult `json:"history,omitempty"`
}

func (m *MonitorStorage) HandleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	req := userRequest{}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" || req.Interval == 0 {
		http.Error(w, "Необходимы значения url и interval", http.StatusBadRequest)
		return
	}

	u, err := url.Parse(req.URL)
	if err != nil {
		http.Error(w, "Неверный формат URL", http.StatusBadRequest)
		return
	}

	if u.Host == "" {
		http.Error(w, "URL должен содержать доменное имя", http.StatusBadRequest)
		return
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		http.Error(w, "Разрешены только http и https", http.StatusBadRequest)
		return
	}

	if req.Interval <= 0 {
		http.Error(w, "interval должно быть целым", http.StatusBadRequest)
		return
	}

	intervalDuration := time.Duration(req.Interval) * time.Second
	newMon, _ := m.Storage.Add(req.URL, intervalDuration)
	status := http.StatusCreated

	m.Scheduler.Schedule(*newMon)

	resp := toUserResponse(newMon)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
}

func (m *MonitorStorage) HandleListMonitors(w http.ResponseWriter, r *http.Request) {
	list := m.Storage.GetAll()
	resp := make([]MonitorResponse, 0, len(list))

	for _, m := range list {
		resp = append(resp, toUserResponse(m))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
}

func (m *MonitorStorage) HandleGetMonitor(w http.ResponseWriter, r *http.Request) {
	reqID := r.PathValue("id")
	id, err := strconv.Atoi(reqID)
	if err != nil {
		http.Error(w, "Ошибка преобразования ID", http.StatusBadRequest)
		return
	}

	if id <= 0 {
		http.Error(w, "ID должен быть больше нуля", http.StatusBadRequest)
		return
	}

	resp, err := m.Storage.Get(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, "404 Not Found", http.StatusNotFound)
			return
		}
		return
	}

	result := toUserResponse(resp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "Internal Error", http.StatusInternalServerError)
		return
	}
}

func (m *MonitorStorage) HandleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	reqID := r.PathValue("id")
	id, err := strconv.Atoi(reqID)
	if err != nil {
		http.Error(w, "Ошибка преобразования ID", http.StatusBadRequest)
		return
	}

	if id <= 0 {
		http.Error(w, "ID должен быть больше нуля", http.StatusBadRequest)
		return
	}

	if isDelete := m.Storage.Delete(id); !isDelete {
		http.Error(w, "404 Not Found", http.StatusNotFound)
		return
	}

	m.Scheduler.Unschedule(id)
	w.WriteHeader(http.StatusNoContent)
}

func (m *MonitorStorage) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	duration := time.Since(startTime)
	uptime := fmt.Sprintf("%dh %dm", int(duration.Hours()), int(duration.Minutes())%60)

	status := map[string]any{
		"status": "ok",
		"uptime": uptime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func toUserResponse(m *models.Monitor) MonitorResponse {
	return MonitorResponse{
		ID:             m.ID,
		Url:            m.Url,
		Interval:       int(m.Interval.Seconds()),
		Status:         m.Status,
		ResponseTimeMs: m.ResponseTimeMs.Milliseconds(),
		LastCheck:      m.LastCheck,
		History:        m.History,
	}
}
