package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
)

type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

var ErrBudgetExceeded = errors.New("превышен бюджет")

var budgets = map[string]Budget{}

func SetBudget(b Budget) error {
	if b.Category == "" {
		return errors.New("категория бюджета не может быть пустой")
	}
	if b.Limit <= 0 {
		return errors.New("лимит бюджета должен быть больше 0")
	}

	budgets[b.Category] = b
	return nil
}

func LoadBudgets(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("не удалось прочитать бюджеты: %w", err)
	}

	var loaded []Budget
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("не удалось разобрать JSON с бюджетами: %w", err)
	}

	for i, b := range loaded {
		if err := SetBudget(b); err != nil {
			return fmt.Errorf("некорректный бюджет #%d: %w", i+1, err)
		}
	}
	return nil
}

func ListBudgets() []Budget {
	return slices.SortedFunc(maps.Values(budgets), func(a, b Budget) int {
		return strings.Compare(a.Category, b.Category)
	})
}

func CategoryTotal(category string) float64 {
	var total float64
	for _, tx := range transactions {
		if tx.Category == category {
			total += tx.Amount
		}
	}
	return total
}

func checkBudget(tx Transaction) error {
	b, ok := budgets[tx.Category]
	if !ok {
		return nil
	}

	total := CategoryTotal(tx.Category) + tx.Amount
	if total > b.Limit {
		return fmt.Errorf("%w по категории «%s»: %.2f из %.2f", ErrBudgetExceeded, tx.Category, total, b.Limit)
	}
	return nil
}
