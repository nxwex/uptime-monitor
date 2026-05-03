package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/nxwex/uptime-monitor/internal/storage"
)

func TestHandleMonitors_Concurrency(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	client := server.Client()

	wg := sync.WaitGroup{}
	requestCount := 100

	doRequest := func(method, url string, body []byte) {
		defer wg.Done()
		req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
		if err != nil {
			t.Errorf("ошибка при отправке запроса: %v", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("ошибка при отправке запроса: %v", err)
			return
		}
		defer resp.Body.Close()
	}

	wg.Add(requestCount * 3)

	for i := 0; i < requestCount; i++ {
		go func(numReq int) {
			data := []byte(fmt.Sprintf(`{"url":"https://kata%d.academy", "interval": 30}`, numReq))
			doRequest(http.MethodPost, server.URL+"/monitors", data)
		}(i)

		go func() {
			doRequest(http.MethodGet, server.URL+"/monitors", nil)
		}()

		go func(id int) {
			url := fmt.Sprintf("%s/monitors/%d", server.URL, id+1)
			doRequest(http.MethodDelete, url, nil)
		}(i)
	}

	wg.Wait()
}

func TestHandleListMonitors(t *testing.T) {
	server, storage := setupTestServer()
	defer server.Close()

	t.Run("обычный запрос", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/monitors")
		if err != nil {
			t.Fatalf("ошибка при GET запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("ожидали 200 статус код, получили %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ошибка при чтении body: %v", err)
		}

		got := string(bytes.TrimSpace(body))

		if got != "[]" {
			t.Errorf("ожидали [], получили %s", got)
		}
	})
	t.Run("запрос двух мониторов", func(t *testing.T) {
		storage.Add("https://google.com", 40)
		storage.Add("https://yandex.ru", 30)

		resp, err := http.Get(server.URL + "/monitors")
		if err != nil {
			t.Fatalf("ошибка при GET запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("ожидали 200 статус код, получили %d", resp.StatusCode)
		}

		var list []any

		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("ошибка декодирования %v", err)
		}

		if len(list) != 2 {
			t.Errorf("ожидали 2 монитора, получили %d", len(list))
		}
	})
}

func TestHandleGetMonitor(t *testing.T) {
	server, storage := setupTestServer()
	defer server.Close()

	t.Run("запрос несуществующего монитора", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/monitors/1")
		if err != nil {
			t.Fatalf("ошибка при GET запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("ожидали 404 статус код, получили %d", resp.StatusCode)
		}
	})

	t.Run("запрос с некорректным ID", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/monitors/sad")
		if err != nil {
			t.Fatalf("ошибка при GET запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("ожидали 400 статус код, получили %d", resp.StatusCode)
		}
	})

	t.Run("запрос с отрицательным ID", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/monitors/-1")
		if err != nil {
			t.Fatalf("ошибка при GET запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("ожидали 400 статус код, получили %d", resp.StatusCode)
		}
	})

	t.Run("запрос существующего монитора", func(t *testing.T) {
		storage.Add("https://kata.academy", 30*time.Second)

		resp, err := http.Get(server.URL + "/monitors/1")
		if err != nil {
			t.Fatalf("ошибка при GET запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("ожидали 200 статус код, получили %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ошибка при чтении body: %v", err)
		}

		got := bytes.TrimSpace(body)
		want := []byte(`{"id":1,"url":"https://kata.academy","interval":30,"status":"active"}`)

		if !reflect.DeepEqual(got, want) {
			t.Errorf("ожидали получить %s, получили %s", want, got)
		}
	})

}

func TestHandleDeleteMonitor(t *testing.T) {
	server, storage := setupTestServer()
	defer server.Close()

	client := http.Client{}

	t.Run("обычный запрос", func(t *testing.T) {
		storage.Add("https://kata.academy", 30*time.Second)

		req, err := http.NewRequest(http.MethodDelete, server.URL+"/monitors/1", nil)
		if err != nil {
			t.Fatalf("ошибка при отправке запроса клиентом: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("ошибка при DELETE запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("ожидали 204 статус код, получили %d", resp.StatusCode)
		}
	})

	t.Run("несуществующий монитор", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, server.URL+"/monitors/1", nil)
		if err != nil {
			t.Fatalf("ошибка при отправке запроса клиентом: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("ошибка при DELETE запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("ожидали 404 статус код, получили %d", resp.StatusCode)
		}
	})

	t.Run("некорректный ID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, server.URL+"/monitors/asd", nil)
		if err != nil {
			t.Fatalf("ошибка при отправке запроса клиентом: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("ошибка при DELETE запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("ожидали 400 статус код, получили %d", resp.StatusCode)
		}
	})

	t.Run("нулевой ID", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, server.URL+"/monitors/0", nil)
		if err != nil {
			t.Fatalf("ошибка при отправке запроса клиентом: %v", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("ошибка при DELETE запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("ожидали 400 статус код, получили %d", resp.StatusCode)
		}
	})
}

func TestHandleCreateMonitor(t *testing.T) {
	server, _ := setupTestServer()
	defer server.Close()

	client := http.Client{}

	t.Run("обычное создание монитора", func(t *testing.T) {
		request := userRequest{
			URL:      "https://kata.academy",
			Interval: 35,
		}

		data, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("ошибка при маршалинге: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, server.URL+"/monitors", bytes.NewBuffer(data))
		if err != nil {
			t.Fatalf("ошибка при отправке запроса клиентом: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("ошибка при POST запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("ожидали 201 статус код, получили %d", resp.StatusCode)
		}
	})
	t.Run("не указан интервал", func(t *testing.T) {
		request := userRequest{
			URL: "https://kata.academy",
		}

		data, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("ошибка при маршалинге: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, server.URL+"/monitors", bytes.NewBuffer(data))
		if err != nil {
			t.Fatalf("ошибка при отправке запроса клиентом: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("ошибка при POST запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("ожидали 400 статус код, получили %d", resp.StatusCode)
		}
	})

	t.Run("неверный url", func(t *testing.T) {
		request := userRequest{
			URL:      "glsadlasld",
			Interval: 50,
		}

		data, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("ошибка при маршалинге: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, server.URL+"/monitors", bytes.NewBuffer(data))
		if err != nil {
			t.Fatalf("ошибка при отправке запроса клиентом: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("ошибка при POST запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("ожидали 400 статус код, получили %d", resp.StatusCode)
		}
	})

	t.Run("битый JSON", func(t *testing.T) {
		data := []byte(`{"ur}`)

		req, err := http.NewRequest(http.MethodPost, server.URL+"/monitors", bytes.NewBuffer(data))
		if err != nil {
			t.Fatalf("ошибка при отправке запроса клиентом: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("ошибка при POST запросе: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("ожидали 400 статус код, получили %d", resp.StatusCode)
		}
	})
}

func setupTestServer() (*httptest.Server, *storage.Monitors) {
	storage := storage.NewMonitors()
	mon := MonitorStorage{Storage: storage}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /monitors", mon.HandleCreateMonitor)
	mux.HandleFunc("GET /monitors", mon.HandleListMonitors)
	mux.HandleFunc("GET /monitors/{id}", mon.HandleGetMonitor)
	mux.HandleFunc("DELETE /monitors/{id}", mon.HandleDeleteMonitor)

	return httptest.NewServer(mux), storage
}
