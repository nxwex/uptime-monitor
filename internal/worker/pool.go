package worker

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/nxwex/uptime-monitor/internal/models"
)

type ResultSaver interface {
	UpdateStatus(id int, status string, duration time.Duration) error
}

type Pool struct {
	tasks chan models.Monitor
	repo  ResultSaver
	wg    sync.WaitGroup
}

func NewPool(workerCount int, repo ResultSaver) *Pool {
	return &Pool{
		tasks: make(chan models.Monitor, workerCount),
		repo:  repo,
	}
}

type Task struct {
	Monitor models.Monitor
	Timeout time.Duration
}

func (p *Pool) worker() {
	defer p.wg.Done()
	client := &http.Client{Timeout: 5 * time.Second}
	var statusCode int
	for monitor := range p.tasks {
		start := time.Now()
		status := "up"
		resp, err := client.Head(monitor.Url)
		if err != nil {
			status = "down"
			log.Printf("ошибка при отправке запроса: %v", err)
		} else {
			if resp.StatusCode >= 400 {
				status = "down"
			}
			statusCode = resp.StatusCode
			if err := resp.Body.Close(); err != nil {
				log.Printf("ошибка при закрытии body: %v", err)
			}
		}
		duration := time.Since(start)

		if err := p.repo.UpdateStatus(monitor.ID, status, duration); err != nil {
			log.Printf("ошибка при обновлении статуса монитора: %v", err)
		}

		log.Printf("Сайт %s, статус: %s [%d], время ответа: %dms", monitor.Url, status, statusCode, duration.Milliseconds())
	}
}

func (p *Pool) Start() {
	for i := 0; i < 5; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

func (p *Pool) Stop() {
	close(p.tasks)
	p.wg.Wait()
}
