package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nxwex/uptime-monitor/internal/handlers"
	"github.com/nxwex/uptime-monitor/internal/storage"
	"github.com/nxwex/uptime-monitor/internal/worker"
)

func main() {
	ctx := context.Background()

	mux := http.NewServeMux()

	storage := storage.NewMonitors()
	workerPool := worker.NewPool(5, storage)
	scheduler := worker.NewScheduler(ctx, workerPool)

	mon := handlers.MonitorStorage{
		Storage:   storage,
		Scheduler: scheduler,
	}

	workerPool.Start()

	for _, m := range storage.GetAll() {
		scheduler.Schedule(*m)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	mux.HandleFunc("POST /api/monitors", mon.HandleCreateMonitor)
	mux.HandleFunc("GET /api/monitors", mon.HandleListMonitors)
	mux.HandleFunc("GET /api/monitors/{id}", mon.HandleGetMonitor)
	mux.HandleFunc("GET /api/health", mon.HandleHealthCheck)
	mux.HandleFunc("DELETE /api/monitors/{id}", mon.HandleDeleteMonitor)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// запуск сервера
	go func() {
		log.Printf("Сервер запущен на порте %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ошибка при запуске сервера: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Сигнал получен %v", sig)
	log.Println("Начинаем graceful shutdown..")

	sdCtx, sdCancel := context.WithTimeout(ctx, 30*time.Second)
	defer sdCancel()

	log.Println("Останавливаем сервер..")
	if err := server.Shutdown(sdCtx); err != nil {
		log.Printf("Ошибка при shutdown: %v", err)
		log.Println("Принудительное закрытие..")
		server.Close()
	}

	log.Println("Останавливаем планировщик..")
	scheduler.Stop()

	log.Println("Останавливаем воркеров..")
	workerPool.Stop()

	log.Println("Сервер остановлен корректно")
}
