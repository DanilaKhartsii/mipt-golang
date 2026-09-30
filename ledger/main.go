package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const budgetsFile = "budgets.json"

func main() {
	fmt.Println("Ledger service started")

	if err := SetBudget(Budget{Category: "Еда", Limit: 5000}); err != nil {
		fmt.Printf("Ошибка установки бюджета: %v\n", err)
	}
	if err := loadBudgetsFromFile(budgetsFile); err != nil {
		fmt.Printf("Ошибка загрузки бюджетов: %v\n", err)
	}
	if err := LoadBudgets(strings.NewReader(`[{"category": "Еда", "limit": "много"}]`)); err != nil {
		fmt.Printf("Ожидаемая ошибка загрузки некорректного JSON: %v\n", err)
	}
	printBudgets(ListBudgets())

	testTransactions := []Transaction{
		{Amount: 350.50, Category: "Еда", Description: "Обед в кафе", Date: time.Now()},
		{Amount: 4500, Category: "Еда", Description: "Продукты на месяц", Date: time.Now()},
		{Amount: 800, Category: "Еда", Description: "Ужин в ресторане", Date: time.Now()},
		{Amount: 1200, Category: "Транспорт", Description: "Проездной на месяц", Date: time.Now()},
		{Amount: 0, Category: "Прочее", Description: "Ошибочная транзакция", Date: time.Now()},
		{Amount: 4990, Category: "Развлечения", Description: "Билеты в кино", Date: time.Now()},
		{Amount: 10000, Category: "Подарки", Description: "Подарок без бюджета", Date: time.Now()},
	}
	for _, tx := range testTransactions {
		addTransaction(tx)
	}
	printBudgets(ListBudgets())
	printTransactions(ListTransactions())

	fmt.Println("Увеличиваем бюджет «Еда» до 6000 и повторяем отклонённую транзакцию")
	if err := SetBudget(Budget{Category: "Еда", Limit: 6000}); err != nil {
		fmt.Printf("Ошибка установки бюджета: %v\n", err)
	}
	addTransaction(testTransactions[2])
	printBudgets(ListBudgets())
}

func loadBudgetsFromFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл %s: %w", path, err)
	}
	defer file.Close()

	return LoadBudgets(bufio.NewReader(file))
}

func addTransaction(tx Transaction) {
	err := AddTransaction(tx)
	switch {
	case errors.Is(err, ErrBudgetExceeded):
		fmt.Printf("Отказ «%s»: %v\n", tx.Description, err)
	case err != nil:
		fmt.Printf("Ошибка добавления «%s»: %v\n", tx.Description, err)
	default:
		fmt.Printf("Добавлена «%s»: %.2f\n", tx.Description, tx.Amount)
	}
}

func printBudgets(budgets []Budget) {
	fmt.Printf("Бюджеты (%d):\n", len(budgets))
	for _, b := range budgets {
		fmt.Printf("  %-12s | %10.2f из %10.2f\n", b.Category, CategoryTotal(b.Category), b.Limit)
	}
}

func printTransactions(transactions []Transaction) {
	fmt.Printf("Транзакции (%d):\n", len(transactions))
	for _, tx := range transactions {
		fmt.Printf("  #%d | %s | %-12s | %10.2f | %s\n",
			tx.ID, tx.Date.Format("2006-01-02"), tx.Category, tx.Amount, tx.Description)
	}
}
