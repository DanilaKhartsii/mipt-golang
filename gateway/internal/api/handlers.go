package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"ledger"
)

type handlers struct{ service ledger.Service }

func (h handlers) createTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, err := req.toTransaction()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.service.AddTransaction(r.Context(), tx)
	var validationErr ledger.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ledger.ErrBudgetExceeded):
		writeError(w, http.StatusConflict, "budget exceeded")
	case err != nil:
		log.Printf("не удалось добавить транзакцию: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	default:
		writeJSON(w, http.StatusCreated, newTransactionResponse(created))
	}
}

func (h handlers) listTransactions(w http.ResponseWriter, r *http.Request) {
	transactions, err := h.service.ListTransactions(r.Context())
	if err != nil {
		log.Printf("не удалось получить транзакции: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, newTransactionResponses(transactions))
}

func (h handlers) createBudget(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	b := req.toBudget()
	if err := h.service.SetBudget(r.Context(), b); err != nil {
		var validationErr ledger.ValidationError
		if errors.As(err, &validationErr) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("не удалось сохранить бюджет: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, newBudgetResponse(b))
}

func (h handlers) listBudgets(w http.ResponseWriter, r *http.Request) {
	budgets, err := h.service.ListBudgets(r.Context())
	if err != nil {
		log.Printf("не удалось получить бюджеты: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, newBudgetResponses(budgets))
}

func (h handlers) reportSummary(w http.ResponseWriter, r *http.Request) {
	from, to, err := parseReportPeriod(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	report, err := h.service.GetReportSummary(r.Context(), from, to)
	if err != nil {
		log.Printf("не удалось построить отчёт: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, newReportSummaryResponse(report))
}

func ping(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "pong"})
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}

func methodNotAllowed(allowed ...string) http.HandlerFunc {
	allow := strings.Join(allowed, ", ")
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", allow)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
