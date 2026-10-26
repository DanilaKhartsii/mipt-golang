package ledger

// Reset очищает хранилище транзакций и бюджетов. Нужна тестам, чтобы сценарии не влияли друг на друга.
func Reset() {
	mu.Lock()
	defer mu.Unlock()

	transactions = []Transaction{}
	budgets = map[string]Budget{}
}
