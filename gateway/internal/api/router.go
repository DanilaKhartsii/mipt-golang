package api

import "net/http"

const apiPrefix = "/api"

func NewRouter() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("POST /transactions", createTransaction)
	api.HandleFunc("GET /transactions", listTransactions)
	api.HandleFunc("/transactions", methodNotAllowed(http.MethodGet, http.MethodPost))
	api.HandleFunc("POST /budgets", createBudget)
	api.HandleFunc("GET /budgets", listBudgets)
	api.HandleFunc("/budgets", methodNotAllowed(http.MethodGet, http.MethodPost))
	api.HandleFunc("GET /reports/summary", reportSummary)
	api.HandleFunc("/reports/summary", methodNotAllowed(http.MethodGet))
	api.HandleFunc("/", notFound)

	mux := http.NewServeMux()
	mux.Handle(apiPrefix+"/", http.StripPrefix(apiPrefix, api))
	mux.HandleFunc("GET /ping", ping)
	mux.HandleFunc("/", notFound)

	return withLogging(withJSONContentType(mux))
}
