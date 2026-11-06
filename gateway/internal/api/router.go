package api

import (
	"net/http"

	"ledger"
)

const apiPrefix = "/api"

func NewRouter(svc ledger.Service) http.Handler {
	h := handlers{service: svc}
	api := http.NewServeMux()
	api.HandleFunc("POST /transactions", h.createTransaction)
	api.HandleFunc("GET /transactions", h.listTransactions)
	api.HandleFunc("/transactions", methodNotAllowed(http.MethodGet, http.MethodPost))
	api.HandleFunc("POST /budgets", h.createBudget)
	api.HandleFunc("GET /budgets", h.listBudgets)
	api.HandleFunc("/budgets", methodNotAllowed(http.MethodGet, http.MethodPost))
	api.HandleFunc("GET /reports/summary", h.reportSummary)
	api.HandleFunc("/reports/summary", methodNotAllowed(http.MethodGet))
	api.HandleFunc("/", notFound)

	mux := http.NewServeMux()
	mux.Handle(apiPrefix+"/", http.StripPrefix(apiPrefix, api))
	mux.HandleFunc("GET /ping", ping)
	mux.HandleFunc("/", notFound)

	return withLogging(withJSONContentType(mux))
}
