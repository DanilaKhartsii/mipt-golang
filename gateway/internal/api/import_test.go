package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"ledger"
)

type importService struct {
	validationService
	importTransactions func(context.Context, []ledger.Transaction, int) (ledger.ImportSummary, error)
}

func (s importService) ImportTransactions(ctx context.Context, transactions []ledger.Transaction, workers int) (ledger.ImportSummary, error) {
	return s.importTransactions(ctx, transactions, workers)
}

func TestImportAPIMapsOriginalIndices(t *testing.T) {
	for _, query := range []string{"", "?workers=1", "?workers=64"} {
		t.Run(query, func(t *testing.T) {
			wantWorkers := ledger.DefaultImportWorkers
			if query == "?workers=1" {
				wantWorkers = 1
			} else if query == "?workers=64" {
				wantWorkers = 64
			}
			type contextKey struct{}
			svc := importService{importTransactions: func(ctx context.Context, transactions []ledger.Transaction, workers int) (ledger.ImportSummary, error) {
				if ctx.Value(contextKey{}) != "import" || workers != wantWorkers {
					t.Errorf("контекст или число воркеров не переданы: workers = %d", workers)
				}
				want := []ledger.Transaction{
					{Amount: 10, Category: "еда", Description: "обед", Date: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)},
					{Amount: 0, Category: "еда", Date: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)},
				}
				if !slices.Equal(transactions, want) {
					t.Errorf("transactions = %+v, want %+v", transactions, want)
				}
				return ledger.ImportSummary{Accepted: 1, Rejected: 1, Errors: []ledger.ImportError{{Index: 1, Error: "сумма транзакции должна быть больше 0"}}}, nil
			}}
			body := `[
				{"amount":1,"category":"еда","date":"не дата"},
				{"amount":10,"category":"еда","description":"обед","date":"2026-11-01"},
				{"amount":0,"category":"еда","date":"2026-11-01"},
				{"amount":1,"category":"еда","date":"2026-02-30"}
			]`
			req := httptest.NewRequest(http.MethodPost, "/api/transactions/bulk"+query, strings.NewReader(body))
			req = req.WithContext(context.WithValue(req.Context(), contextKey{}, "import"))
			rec := httptest.NewRecorder()
			NewRouter(svc, DefaultRequestTimeout).ServeHTTP(rec, req)
			checkResponse(t, rec, http.StatusOK)
			got := decodeResponse[ImportSummaryResponse](t, rec)
			if got.Accepted != 1 || got.Rejected != 3 || len(got.Errors) != 3 {
				t.Fatalf("response = %+v", got)
			}
			for i, index := range []int{0, 2, 3} {
				if got.Errors[i].Index != index || got.Errors[i].Error == "" {
					t.Errorf("Errors[%d] = %+v, want index %d and error", i, got.Errors[i], index)
				}
			}
		})
	}
}

func TestImportAPIEmptyAndAllRejected(t *testing.T) {
	for _, body := range []string{"[]", `[{"date":"invalid"}]`} {
		t.Run(body, func(t *testing.T) {
			svc := importService{importTransactions: func(_ context.Context, transactions []ledger.Transaction, _ int) (ledger.ImportSummary, error) {
				if len(transactions) != 0 {
					t.Errorf("transactions = %+v, want empty", transactions)
				}
				return ledger.ImportSummary{}, nil
			}}
			rec := httptest.NewRecorder()
			NewRouter(svc, DefaultRequestTimeout).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/transactions/bulk", strings.NewReader(body)))
			checkResponse(t, rec, http.StatusOK)
			got := decodeResponse[ImportSummaryResponse](t, rec)
			wantRejected := int64(0)
			if body != "[]" {
				wantRejected = 1
			}
			if got.Accepted != 0 || got.Rejected != wantRejected || got.Errors == nil || int64(len(got.Errors)) != wantRejected {
				t.Fatalf("response = %+v", got)
			}
		})
	}
}

func TestImportAPIBadRequests(t *testing.T) {
	tests := []struct {
		name  string
		query string
		body  string
	}{
		{name: "empty_body"},
		{name: "null", body: "null"},
		{name: "object", body: `{}`},
		{name: "invalid_json", body: `[{`},
		{name: "unknown_field", body: `[{"currency":"RUB"}]`},
		{name: "wrong_type", body: `[{"amount":"10"}]`},
		{name: "workers_empty", query: "?workers=", body: `[]`},
		{name: "workers_zero", query: "?workers=0", body: `[]`},
		{name: "workers_negative", query: "?workers=-1", body: `[]`},
		{name: "workers_fraction", query: "?workers=1.5", body: `[]`},
		{name: "workers_text", query: "?workers=many", body: `[]`},
		{name: "workers_limit", query: "?workers=65", body: `[]`},
		{name: "workers_overflow", query: "?workers=99999999999999999999999999", body: `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			// nil-сервис подтверждает, что неверный запрос не вызывает Ledger.
			NewRouter(nil, DefaultRequestTimeout).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/transactions/bulk"+tt.query, strings.NewReader(tt.body)))
			checkError(t, rec, http.StatusBadRequest)
		})
	}
}

func TestImportAPIServiceErrors(t *testing.T) {
	for _, serviceErr := range []error{context.Canceled, context.DeadlineExceeded, errors.New("database failure")} {
		t.Run(serviceErr.Error(), func(t *testing.T) {
			svc := importService{importTransactions: func(context.Context, []ledger.Transaction, int) (ledger.ImportSummary, error) {
				return ledger.ImportSummary{Accepted: 1}, fmt.Errorf("import: %w", serviceErr)
			}}
			rec := httptest.NewRecorder()
			NewRouter(svc, DefaultRequestTimeout).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/transactions/bulk", strings.NewReader("[]")))
			status := http.StatusInternalServerError
			if errors.Is(serviceErr, context.Canceled) || errors.Is(serviceErr, context.DeadlineExceeded) {
				status = http.StatusGatewayTimeout
			}
			checkError(t, rec, status)
			if status == http.StatusGatewayTimeout && decodeResponse[errorResponse](t, rec).Error != "request timed out or canceled" {
				t.Fatalf("response = %s", rec.Body)
			}
		})
	}
}

func TestImportAPIMethodNotAllowed(t *testing.T) {
	rec := doRequest(t, http.MethodGet, "/api/transactions/bulk", "")
	checkError(t, rec, http.StatusMethodNotAllowed)
	if rec.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("Allow = %q, want POST", rec.Header().Get("Allow"))
	}
}

func TestImportAPIWithDatabase(t *testing.T) {
	resetLedger(t)
	mustCreateBudget(t, `{"category":"еда","limit":100}`)
	rec := doRequest(t, http.MethodPost, "/api/transactions/bulk?workers=4", `[
		{"amount":1,"category":"еда","date":"invalid"},
		{"amount":100,"category":"еда","date":"2026-11-01"},
		{"amount":101,"category":"еда","date":"2026-11-01"},
		{"amount":0,"category":"еда","date":"2026-11-01"},
		{"amount":1000,"category":"транспорт","date":"2026-11-01"}
	]`)
	checkResponse(t, rec, http.StatusOK)
	got := decodeResponse[ImportSummaryResponse](t, rec)
	if got.Accepted != 2 || got.Rejected != 3 || len(got.Errors) != 3 {
		t.Fatalf("response = %+v", got)
	}
	for i, index := range []int{0, 2, 3} {
		if got.Errors[i].Index != index || got.Errors[i].Error == "" {
			t.Errorf("Errors[%d] = %+v, want index %d", i, got.Errors[i], index)
		}
	}
	transactions, err := fixture.Service.ListTransactions(context.Background())
	if err != nil || len(transactions) != 2 {
		t.Fatalf("ListTransactions() = %+v, %v", transactions, err)
	}
}
