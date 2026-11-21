package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ledger"
)

func TestTimeoutForwardsContextAndResponse(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "value")
	var handlerContext context.Context
	handler := withJSONContentType(withTimeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerContext = r.Context()
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > DefaultRequestTimeout || r.Context().Value(contextKey{}) != "value" {
			t.Error("контекст запроса не передан или не установлен deadline")
		}
		w.Header().Set("X-Result", "ok")
		writeJSON(w, http.StatusCreated, map[string]string{"result": "ok"})
	}), DefaultRequestTimeout))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	checkResponse(t, rec, http.StatusCreated)
	if rec.Header().Get("X-Result") != "ok" || decodeResponse[map[string]string](t, rec)["result"] != "ok" {
		t.Fatalf("response = %v %s", rec.Header(), rec.Body)
	}
	if handlerContext.Err() != context.Canceled {
		t.Error("контекст не освобождён после завершения обработчика")
	}
}

func TestTimeoutDiscardsLateResponse(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})
	t.Cleanup(func() {
		close(release)
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("обработчик не завершился")
		}
	})
	handler := withJSONContentType(withTimeout(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		w.Header().Set("X-Partial", "discard")
		writeJSON(w, http.StatusOK, map[string]string{"partial": "discard"})
		<-r.Context().Done()
		<-release
		writeJSON(w, http.StatusOK, map[string]string{"late": "discard"})
	}), 20*time.Millisecond))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	checkError(t, rec, http.StatusGatewayTimeout)
	if got := decodeResponse[errorResponse](t, rec).Error; got != "request timed out or canceled" {
		t.Fatalf("error = %q", got)
	}
	if rec.Header().Get("X-Partial") != "" || strings.Contains(rec.Body.String(), "discard") {
		t.Fatalf("частичный ответ попал клиенту: %v %s", rec.Header(), rec.Body)
	}
}

type waitingService struct {
	validationService
	finished chan struct{}
}

func (s waitingService) wait(ctx context.Context) error {
	defer close(s.finished)
	<-ctx.Done()
	return ctx.Err()
}

func (s waitingService) SetBudget(ctx context.Context, _ ledger.Budget) error {
	return s.wait(ctx)
}

func (s waitingService) AddTransaction(ctx context.Context, _ ledger.Transaction) (ledger.Transaction, error) {
	return ledger.Transaction{}, s.wait(ctx)
}

func (s waitingService) ImportTransactions(ctx context.Context, _ []ledger.Transaction, _ int) (ledger.ImportSummary, error) {
	return ledger.ImportSummary{}, s.wait(ctx)
}

func (s waitingService) ListBudgets(ctx context.Context) ([]ledger.Budget, error) {
	return nil, s.wait(ctx)
}

func (s waitingService) ListTransactions(ctx context.Context) ([]ledger.Transaction, error) {
	return nil, s.wait(ctx)
}

func (s waitingService) GetReportSummary(ctx context.Context, _, _ time.Time) (ledger.ReportSummary, error) {
	return ledger.ReportSummary{}, s.wait(ctx)
}

func TestTimeoutAppliesToEveryAPIRoute(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/transactions"},
		{method: http.MethodPost, path: "/api/transactions", body: `{"amount":1,"category":"еда","date":"2026-10-01"}`},
		{method: http.MethodPost, path: "/api/transactions/bulk", body: `[{"amount":1,"category":"еда","date":"2026-10-01"}]`},
		{method: http.MethodGet, path: "/api/budgets"},
		{method: http.MethodPost, path: "/api/budgets", body: `{"category":"еда","limit":100}`},
		{method: http.MethodGet, path: "/api/reports/summary?from=2026-10-01&to=2026-10-31"},
	}
	for _, tt := range tests {
		t.Run(tt.method+tt.path, func(t *testing.T) {
			svc := waitingService{finished: make(chan struct{})}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			NewRouter(svc, 20*time.Millisecond).ServeHTTP(rec, req)
			checkError(t, rec, http.StatusGatewayTimeout)
			select {
			case <-svc.finished:
			case <-time.After(time.Second):
				t.Fatal("операция сервиса не завершилась после отмены")
			}
		})
	}
}

func TestCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/budgets", nil).WithContext(ctx)
	NewRouter(nil, DefaultRequestTimeout).ServeHTTP(rec, req)
	checkError(t, rec, http.StatusGatewayTimeout)
}

type failedReportService struct {
	validationService
	err error
}

func (s failedReportService) GetReportSummary(context.Context, time.Time, time.Time) (ledger.ReportSummary, error) {
	return ledger.ReportSummary{}, s.err
}

func TestReportContextErrors(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/reports/summary?from=2026-10-01&to=2026-10-31", nil)
			svc := failedReportService{err: fmt.Errorf("report: %w", err)}
			NewRouter(svc, DefaultRequestTimeout).ServeHTTP(rec, req)
			checkError(t, rec, http.StatusGatewayTimeout)
		})
	}
}

func TestTimeoutPropagatesPanic(t *testing.T) {
	defer func() {
		if got := recover(); got != "handler panic" {
			t.Errorf("panic = %v, want handler panic", got)
		}
	}()
	handler := withTimeout(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("handler panic")
	}), DefaultRequestTimeout)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
