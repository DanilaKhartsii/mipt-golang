package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"ledger"
)

const jsonContentType = "application/json; charset=utf-8"

// Тесты работают с общим хранилищем ledger, поэтому не параллельные и сбрасывают состояние в каждом подтесте.

func TestBudgetsAPI(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		resetLedger(t)

		rec := doRequest(t, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
		checkResponse(t, rec, http.StatusCreated)

		want := BudgetResponse{Category: "еда", Limit: 5000}
		if got := decodeResponse[BudgetResponse](t, rec); got != want {
			t.Errorf("body = %+v, want %+v", got, want)
		}
	})

	t.Run("list", func(t *testing.T) {
		resetLedger(t)
		mustCreateBudget(t, `{"category":"еда","limit":5000}`)

		rec := doRequest(t, http.MethodGet, "/api/budgets", "")
		checkResponse(t, rec, http.StatusOK)

		want := BudgetResponse{Category: "еда", Limit: 5000}
		if got := decodeResponse[[]BudgetResponse](t, rec); !slices.Contains(got, want) {
			t.Errorf("body = %+v, want to contain %+v", got, want)
		}
	})

	badRequests := []struct {
		name string
		body string
	}{
		{name: "bad_json", body: `{"category":"еда",`},
		{name: "unknown_field", body: `{"category":"еда","limit":5000,"currency":"RUB"}`},
		{name: "zero_limit", body: `{"category":"еда","limit":0}`},
		{name: "negative_limit", body: `{"category":"еда","limit":-100}`},
		{name: "empty_category", body: `{"category":"","limit":5000}`},
	}
	for _, tt := range badRequests {
		t.Run(tt.name, func(t *testing.T) {
			resetLedger(t)

			rec := doRequest(t, http.MethodPost, "/api/budgets", tt.body)
			checkError(t, rec, http.StatusBadRequest)
		})
	}
}

func TestTransactionsAPI(t *testing.T) {
	const validTransaction = `{"amount":1500,"category":"еда","description":"продукты","date":"2026-10-26"}`
	wantTransaction := TransactionResponse{
		ID:          1,
		Amount:      1500,
		Category:    "еда",
		Description: "продукты",
		Date:        "2026-10-26",
	}

	t.Run("ok", func(t *testing.T) {
		resetLedger(t)
		mustCreateBudget(t, `{"category":"еда","limit":5000}`)

		rec := doRequest(t, http.MethodPost, "/api/transactions", validTransaction)
		checkResponse(t, rec, http.StatusCreated)

		if got := decodeResponse[TransactionResponse](t, rec); got != wantTransaction {
			t.Errorf("body = %+v, want %+v", got, wantTransaction)
		}
	})

	t.Run("list", func(t *testing.T) {
		resetLedger(t)
		mustCreateBudget(t, `{"category":"еда","limit":5000}`)
		checkResponse(t, doRequest(t, http.MethodPost, "/api/transactions", validTransaction), http.StatusCreated)

		rec := doRequest(t, http.MethodGet, "/api/transactions", "")
		checkResponse(t, rec, http.StatusOK)

		want := []TransactionResponse{wantTransaction}
		if got := decodeResponse[[]TransactionResponse](t, rec); !slices.Equal(got, want) {
			t.Errorf("body = %+v, want %+v", got, want)
		}
	})

	t.Run("exceeded", func(t *testing.T) {
		resetLedger(t)
		mustCreateBudget(t, `{"category":"еда","limit":2000}`)
		checkResponse(t, doRequest(t, http.MethodPost, "/api/transactions", validTransaction), http.StatusCreated)

		rec := doRequest(t, http.MethodPost, "/api/transactions", validTransaction)
		checkResponse(t, rec, http.StatusConflict)

		want := errorResponse{Error: "budget exceeded"}
		if got := decodeResponse[errorResponse](t, rec); got != want {
			t.Errorf("body = %+v, want %+v", got, want)
		}
		if got := len(ledger.ListTransactions()); got != 1 {
			t.Errorf("len(ledger.ListTransactions()) = %d, want 1", got)
		}
	})

	badRequests := []struct {
		name string
		body string
	}{
		{name: "bad_json", body: `{"amount":1500,`},
		{name: "invalid_date", body: `{"amount":1500,"category":"еда","date":"26.10.2026"}`},
		{name: "zero_amount", body: `{"amount":0,"category":"еда","date":"2026-10-26"}`},
		{name: "empty_category", body: `{"amount":1500,"category":"","date":"2026-10-26"}`},
		{name: "missing_date", body: `{"amount":1500,"category":"еда"}`},
	}
	for _, tt := range badRequests {
		t.Run(tt.name, func(t *testing.T) {
			resetLedger(t)

			rec := doRequest(t, http.MethodPost, "/api/transactions", tt.body)
			checkError(t, rec, http.StatusBadRequest)
		})
	}
}

func resetLedger(t *testing.T) {
	t.Helper()

	ledger.Reset()
	t.Cleanup(ledger.Reset)
}

func mustCreateBudget(t *testing.T, body string) {
	t.Helper()

	checkResponse(t, doRequest(t, http.MethodPost, "/api/budgets", body), http.StatusCreated)
}

func doRequest(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, req)
	return rec
}

func checkResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, wantStatus, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != jsonContentType {
		t.Errorf("Content-Type = %q, want %q", got, jsonContentType)
	}
}

func checkError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) {
	t.Helper()

	checkResponse(t, rec, wantStatus)
	if got := decodeResponse[errorResponse](t, rec); got.Error == "" {
		t.Errorf("body = %s, want non-empty error", rec.Body)
	}
}

func decodeResponse[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("не удалось разобрать JSON ответа %q: %v", rec.Body, err)
	}
	return v
}
