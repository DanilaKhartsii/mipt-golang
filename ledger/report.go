package ledger

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"
)

type CategorySummary struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

type ReportSummary struct {
	From       time.Time         `json:"from"`
	To         time.Time         `json:"to"`
	Categories []CategorySummary `json:"categories"`
	Total      float64           `json:"total"`
}

const reportCacheTTL = 30 * time.Second

// GetReportSummary возвращает расходы по категориям за период [from, to] включительно.
// Результат кешируется в Redis на reportCacheTTL, поэтому новые транзакции попадают в отчёт с задержкой до TTL.
func GetReportSummary(ctx context.Context, from, to time.Time) (ReportSummary, error) {
	key := fmt.Sprintf("report:summary:%s:%s", from.Format(time.DateOnly), to.Format(time.DateOnly))

	var cached ReportSummary
	if readCache(ctx, key, &cached) {
		log.Printf("Ledger: отчёт %s взят из кеша", key)
		return cached, nil
	}

	rows, err := sqlDB.QueryContext(ctx,
		`SELECT category, SUM(amount) FROM expenses WHERE date BETWEEN $1 AND $2 GROUP BY category ORDER BY category`,
		from, to,
	)
	if err != nil {
		return ReportSummary{}, fmt.Errorf("не удалось построить отчёт: %w", err)
	}
	defer rows.Close()

	report := ReportSummary{From: from, To: to, Categories: []CategorySummary{}}
	for rows.Next() {
		var s CategorySummary
		if err := rows.Scan(&s.Category, &s.Total); err != nil {
			return ReportSummary{}, fmt.Errorf("не удалось прочитать строку отчёта: %w", err)
		}
		report.Categories = append(report.Categories, s)
		report.Total += s.Total
	}
	if err := rows.Err(); err != nil {
		return ReportSummary{}, fmt.Errorf("не удалось построить отчёт: %w", err)
	}
	// Округляем до копеек, чтобы сумма float64 не давала хвостов вроде 0.30000000000000004.
	report.Total = math.Round(report.Total*100) / 100

	log.Printf("Ledger: отчёта %s нет в кеше, посчитан по БД", key)
	writeCache(ctx, key, report, reportCacheTTL)
	return report, nil
}
