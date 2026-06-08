package main

import (
	"chat/globals"
	"chat/utils"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
)

const modelAnalysisKeyTTL = time.Hour * 24 * 14

type noopCharge struct{}

func (noopCharge) GetType() string           { return globals.NonBilling }
func (noopCharge) GetModels() []string       { return nil }
func (noopCharge) GetInput() float32         { return 0 }
func (noopCharge) GetOutput() float32        { return 0 }
func (noopCharge) SupportAnonymous() bool    { return true }
func (noopCharge) IsBilling() bool           { return false }
func (noopCharge) IsBillingType(string) bool { return false }
func (noopCharge) GetLimit() float32         { return 0 }

type recordedPrompts struct {
	Model    string            `json:"model,omitempty"`
	Messages []globals.Message `json:"messages,omitempty"`
}

type modelCallRecord struct {
	ID           int64
	Model        string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	Input        string
	Output       string
	CreatedAt    time.Time
}

type repairResult struct {
	ID              int64
	Model           string
	Day             string
	OldInputTokens  int
	NewInputTokens  int
	OldOutputTokens int
	NewOutputTokens int
	OldTotalTokens  int
	NewTotalTokens  int
}

type summaryRow struct {
	Day   string
	Model string
	Rows  int
	Delta int64
}

func main() {
	var (
		mysqlDSN      = flag.String("mysql-dsn", "", "MySQL DSN, for example user:pass@tcp(host:3306)/chatnio")
		redisAddr     = flag.String("redis-addr", "", "Redis address, for example 127.0.0.1:6379")
		redisPassword = flag.String("redis-password", "", "Redis password")
		redisDB       = flag.Int("redis-db", 0, "Redis DB index")
		startDate     = flag.String("start-date", "", "Start date in YYYY-MM-DD")
		endDate       = flag.String("end-date", "", "End date in YYYY-MM-DD, exclusive")
		apply         = flag.Bool("apply", false, "Apply updates to MySQL and Redis; default is dry-run")
		imagesOnly    = flag.Bool("images-only", true, "Only scan image-generation models")
	)
	flag.Parse()

	if *mysqlDSN == "" || *redisAddr == "" || *startDate == "" || *endDate == "" {
		flag.Usage()
		os.Exit(2)
	}

	start, err := time.Parse("2006-01-02", *startDate)
	if err != nil {
		fail("invalid start-date: %v", err)
	}
	end, err := time.Parse("2006-01-02", *endDate)
	if err != nil {
		fail("invalid end-date: %v", err)
	}
	if !start.Before(end) {
		fail("start-date must be before end-date")
	}

	db, err := sql.Open("mysql", *mysqlDSN)
	if err != nil {
		fail("open mysql: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fail("ping mysql: %v", err)
	}

	cache := redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
		Password: *redisPassword,
		DB:       *redisDB,
	})
	defer cache.Close()
	if err := cache.Ping(context.Background()).Err(); err != nil {
		fail("ping redis: %v", err)
	}

	records, err := loadRecords(db, start, end, *imagesOnly)
	if err != nil {
		fail("load records: %v", err)
	}

	repairs, err := computeRepairs(records)
	if err != nil {
		fail("compute repairs: %v", err)
	}

	printSummary(records, repairs, *apply)
	if len(repairs) == 0 {
		return
	}

	if !*apply {
		return
	}

	if err := applyRepairs(db, cache, repairs); err != nil {
		fail("apply repairs: %v", err)
	}

	fmt.Printf("applied %d repair(s)\n", len(repairs))
}

func loadRecords(db *sql.DB, start, end time.Time, imagesOnly bool) ([]modelCallRecord, error) {
	query := `
		SELECT id, model, input_tokens, output_tokens, total_tokens, input, output, created_at
		FROM model_call_record
		WHERE created_at >= ? AND created_at < ?
	`
	args := []interface{}{start, end}
	if imagesOnly {
		query += `
			AND (
				LOWER(model) LIKE '%image%' OR
				LOWER(model) LIKE 'imagen-%' OR
				LOWER(model) LIKE 'grok-imagine-image%'
			)
		`
	}
	query += ` ORDER BY created_at ASC, id ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []modelCallRecord
	for rows.Next() {
		var (
			record       modelCallRecord
			inputRaw     sql.NullString
			outputRaw    sql.NullString
			createdAtRaw []byte
		)
		if err := rows.Scan(
			&record.ID,
			&record.Model,
			&record.InputTokens,
			&record.OutputTokens,
			&record.TotalTokens,
			&inputRaw,
			&outputRaw,
			&createdAtRaw,
		); err != nil {
			return nil, err
		}

		record.Input = inputRaw.String
		record.Output = outputRaw.String
		record.CreatedAt, err = parseMySQLDateTime(createdAtRaw)
		if err != nil {
			return nil, fmt.Errorf("parse created_at for record %d: %w", record.ID, err)
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

func computeRepairs(records []modelCallRecord) ([]repairResult, error) {
	repairs := make([]repairResult, 0)
	for _, record := range records {
		result, err := recomputeRecord(record)
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", record.ID, err)
		}
		if result == nil {
			continue
		}
		repairs = append(repairs, *result)
	}
	return repairs, nil
}

func recomputeRecord(record modelCallRecord) (*repairResult, error) {
	inputTokens, err := countInputTokensFromRecordedPrompts(record.Model, record.Input)
	if err != nil {
		return nil, err
	}

	outputTokens := utils.NumTokensFromResponse(record.Output, record.Model)
	totalTokens := inputTokens + outputTokens

	if record.InputTokens == inputTokens &&
		record.OutputTokens == outputTokens &&
		record.TotalTokens == totalTokens {
		return nil, nil
	}

	return &repairResult{
		ID:              record.ID,
		Model:           record.Model,
		Day:             record.CreatedAt.Format("2006-01-02"),
		OldInputTokens:  record.InputTokens,
		NewInputTokens:  inputTokens,
		OldOutputTokens: record.OutputTokens,
		NewOutputTokens: outputTokens,
		OldTotalTokens:  record.TotalTokens,
		NewTotalTokens:  totalTokens,
	}, nil
}

func countInputTokensFromRecordedPrompts(model, raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	var prompts recordedPrompts
	if err := json.Unmarshal([]byte(raw), &prompts); err != nil {
		return 0, fmt.Errorf("decode prompts: %w", err)
	}

	buffer := utils.NewBuffer(model, prompts.Messages, noopCharge{})
	return buffer.CountInputToken(), nil
}

func parseMySQLDateTime(raw []byte) (time.Time, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return time.Time{}, fmt.Errorf("empty datetime")
	}

	for _, layout := range []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported datetime %q", value)
}

func printSummary(records []modelCallRecord, repairs []repairResult, apply bool) {
	mode := "dry-run"
	if apply {
		mode = "apply"
	}

	fmt.Printf("mode: %s\n", mode)
	fmt.Printf("scanned rows: %d\n", len(records))
	fmt.Printf("rows needing repair: %d\n", len(repairs))

	if len(repairs) == 0 {
		return
	}

	summaries := summarizeRepairs(repairs)
	fmt.Println("redis delta by day/model:")
	for _, summary := range summaries {
		fmt.Printf("  %s %s rows=%d delta=%d\n", summary.Day, summary.Model, summary.Rows, summary.Delta)
	}

	fmt.Println("sample repaired rows:")
	limit := len(repairs)
	if limit > 10 {
		limit = 10
	}
	for _, repair := range repairs[:limit] {
		fmt.Printf(
			"  id=%d model=%s input %d->%d output %d->%d total %d->%d\n",
			repair.ID,
			repair.Model,
			repair.OldInputTokens,
			repair.NewInputTokens,
			repair.OldOutputTokens,
			repair.NewOutputTokens,
			repair.OldTotalTokens,
			repair.NewTotalTokens,
		)
	}
}

func summarizeRepairs(repairs []repairResult) []summaryRow {
	grouped := make(map[string]*summaryRow)
	for _, repair := range repairs {
		key := repair.Day + "\x00" + repair.Model
		row, ok := grouped[key]
		if !ok {
			row = &summaryRow{Day: repair.Day, Model: repair.Model}
			grouped[key] = row
		}
		row.Rows++
		row.Delta += int64(repair.NewTotalTokens - repair.OldTotalTokens)
	}

	summaries := make([]summaryRow, 0, len(grouped))
	for _, row := range grouped {
		summaries = append(summaries, *row)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Day != summaries[j].Day {
			return summaries[i].Day < summaries[j].Day
		}
		return summaries[i].Model < summaries[j].Model
	})

	return summaries
}

func applyRepairs(db *sql.DB, cache *redis.Client, repairs []repairResult) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, repair := range repairs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE model_call_record
			SET input_tokens = ?, output_tokens = ?, total_tokens = ?
			WHERE id = ?
		`, repair.NewInputTokens, repair.NewOutputTokens, repair.NewTotalTokens, repair.ID); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	for _, summary := range summarizeRepairs(repairs) {
		key := getModelAnalysisKey(summary.Day, summary.Model)
		if err := cache.IncrBy(ctx, key, summary.Delta).Err(); err != nil {
			return err
		}

		if ttl := cache.TTL(ctx, key).Val(); ttl < 0 {
			if err := cache.Expire(ctx, key, modelAnalysisKeyTTL).Err(); err != nil {
				return err
			}
		}
	}

	return nil
}

func getModelAnalysisKey(day, model string) string {
	return fmt.Sprintf("nio:model-analysis-%s-%s", model, day)
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
