package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"ledger"
)

const budgetsFile = "budgets.json"

func main() {
	fmt.Println("Ledger service started")

	fmt.Println("Проверка CheckValid для разных типов:")
	checkValid("Транзакция «Кофе»", ledger.Transaction{Amount: 250, Category: "Еда", Description: "Кофе", Date: time.Now()})
	checkValid("Транзакция без даты", ledger.Transaction{Amount: 250, Category: "Еда", Description: "Кофе"})
	checkValid("Бюджет «Спорт»", ledger.Budget{Category: "Спорт", Limit: 2000})
	checkValid("Бюджет с отрицательным лимитом", ledger.Budget{Category: "Спорт", Limit: -100})

	if err := ledger.SetBudget(ledger.Budget{Category: "Еда", Limit: 5000}); err != nil {
		fmt.Printf("Ошибка установки бюджета: %v\n", err)
	}
	if err := ledger.SetBudget(ledger.Budget{Category: "", Limit: 1000}); err != nil {
		fmt.Printf("Ожидаемая ошибка установки бюджета: %v\n", err)
	}
	if err := loadBudgetsFromFile(budgetsFile); err != nil {
		fmt.Printf("Ошибка загрузки бюджетов: %v\n", err)
	}
	if err := ledger.LoadBudgets(strings.NewReader(`[{"category": "Еда", "limit": "много"}]`)); err != nil {
		fmt.Printf("Ожидаемая ошибка загрузки некорректного JSON: %v\n", err)
	}
	printBudgets(ledger.ListBudgets())

	testTransactions := []ledger.Transaction{
		{Amount: 350.50, Category: "Еда", Description: "Обед в кафе", Date: time.Now()},
		{Amount: 4500, Category: "Еда", Description: "Продукты на месяц", Date: time.Now()},
		{Amount: 800, Category: "Еда", Description: "Ужин в ресторане", Date: time.Now()},
		{Amount: 1200, Category: "Транспорт", Description: "Проездной на месяц", Date: time.Now()},
		{Amount: 0, Category: "Прочее", Description: "Ошибочная транзакция", Date: time.Now()},
		{Amount: -300, Category: "Прочее", Description: "Отрицательная сумма", Date: time.Now()},
		{Amount: 300, Category: "", Description: "Транзакция без категории", Date: time.Now()},
		{Amount: 4990, Category: "Развлечения", Description: "Билеты в кино", Date: time.Now()},
		{Amount: 10000, Category: "Подарки", Description: "Подарок без бюджета", Date: time.Now()},
	}
	for _, tx := range testTransactions {
		addTransaction(tx)
	}
	printBudgets(ledger.ListBudgets())
	printTransactions(ledger.ListTransactions())

	fmt.Println("Увеличиваем бюджет «Еда» до 6000 и повторяем отклонённую транзакцию")
	if err := ledger.SetBudget(ledger.Budget{Category: "Еда", Limit: 6000}); err != nil {
		fmt.Printf("Ошибка установки бюджета: %v\n", err)
	}
	addTransaction(testTransactions[2])
	printBudgets(ledger.ListBudgets())
}

func loadBudgetsFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл %s: %w", path, err)
	}
	defer file.Close()

	return ledger.LoadBudgets(bufio.NewReader(file))
}

func checkValid(name string, v ledger.Validatable) {
	if err := ledger.CheckValid(v); err != nil {
		fmt.Printf("  %s: некорректно — %v\n", name, err)
		return
	}
	fmt.Printf("  %s: корректно\n", name)
}

func addTransaction(tx ledger.Transaction) {
	_, err := ledger.AddTransaction(tx)
	switch {
	case errors.Is(err, ledger.ErrBudgetExceeded):
		fmt.Printf("Отказ «%s»: %v\n", tx.Description, err)
	case err != nil:
		fmt.Printf("Ошибка добавления «%s»: %v\n", tx.Description, err)
	default:
		fmt.Printf("Добавлена «%s»: %.2f\n", tx.Description, tx.Amount)
	}
}

func printBudgets(budgets []ledger.Budget) {
	fmt.Printf("Бюджеты (%d):\n", len(budgets))
	for _, b := range budgets {
		fmt.Printf("  %-12s | %10.2f из %10.2f\n", b.Category, ledger.CategoryTotal(b.Category), b.Limit)
	}
}

func printTransactions(transactions []ledger.Transaction) {
	fmt.Printf("Транзакции (%d):\n", len(transactions))
	for _, tx := range transactions {
		fmt.Printf("  #%d | %s | %-12s | %10.2f | %s\n",
			tx.ID, tx.Date.Format("2006-01-02"), tx.Category, tx.Amount, tx.Description)
	}
}
