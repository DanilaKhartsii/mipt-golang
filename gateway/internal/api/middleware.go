package api

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"
)

const DefaultRequestTimeout = 2 * time.Second

func withTimeout(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		if ctx.Err() != nil {
			writeRequestTimeout(w)
			return
		}

		response := &bufferedResponse{header: w.Header().Clone()}
		done := make(chan struct{})
		panics := make(chan any, 1)
		go func() {
			defer func() {
				if p := recover(); p != nil {
					panics <- p
				}
			}()
			next.ServeHTTP(response, r.WithContext(ctx))
			close(done)
		}()

		select {
		case p := <-panics:
			panic(p)
		case <-ctx.Done():
			writeRequestTimeout(w)
		case <-done:
			if ctx.Err() != nil {
				writeRequestTimeout(w)
				return
			}
			// Буфер читает только эта горутина и только после завершения обработчика.
			for key, values := range response.header {
				w.Header()[key] = values
			}
			status := response.status
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			if _, err := w.Write(response.body.Bytes()); err != nil {
				log.Printf("не удалось записать ответ: %v", err)
			}
		}
	})
}

// bufferedResponse не отправляет клиенту частичный ответ до проверки таймаута.
// После отмены обработчик может завершить запись в свой буфер, не затрагивая w.
type bufferedResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *bufferedResponse) Header() http.Header { return w.header }

func (w *bufferedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *bufferedResponse) Write(data []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	return w.body.Write(data)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func withJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}
