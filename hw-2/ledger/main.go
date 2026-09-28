package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("Ledger service started")

	testTransactions := []Transaction{
		{Amount: 350.50, Category: "Еда", Description: "Обед в кафе", Date: time.Now()},
		{Amount: 1200, Category: "Транспорт", Description: "Проездной на месяц", Date: time.Now()},
		{Amount: 0, Category: "Прочее", Description: "Ошибочная транзакция", Date: time.Now()},
		{Amount: 4990, Category: "Развлечения", Description: "Билеты в кино", Date: time.Now()},
	}
	for _, tx := range testTransactions {
		if err := AddTransaction(tx); err != nil {
			fmt.Printf("Ошибка добавления «%s»: %v\n", tx.Description, err)
		}
	}

	printTransactions(ListTransactions())
}

func printTransactions(transactions []Transaction) {
	fmt.Printf("Транзакции (%d):\n", len(transactions))
	for _, tx := range transactions {
		fmt.Printf("  #%d | %s | %-12s | %10.2f | %s\n",
			tx.ID, tx.Date.Format("2006-01-02"), tx.Category, tx.Amount, tx.Description)
	}
}
