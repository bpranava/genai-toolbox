package duckdb

import (
	"context"
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/googleapis/genai-toolbox/internal/testutils"
	"github.com/googleapis/genai-toolbox/tests"
)

var (
	DuckDbKind = "duckdb-sql"
	dbPath     = "/tmp/hotel_test.db"
)

func getDuckDbVars() map[string]any {
	return map[string]any{
		"kind":       "duckdb",
		"dbFilePath": dbPath,
		"configurations": map[string]any{
			"access_mode": "READ_ONLY",
		},
	}
}

func setupDuckDb(t *testing.T) {
	os.Setenv("CLIENT_ID", "12321")

	// Remove any existing database file to ensure a clean state
	os.Remove(dbPath)

	// Open a connection to DuckDB
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		t.Fatalf("Failed to open DuckDB connection: %v", err)
	}
	defer db.Close()

	// Create the hotel_bookings table
	_, err = db.Exec(`
		CREATE TABLE hotel_bookings (
			booking_id INTEGER,
			guest_name VARCHAR,
			check_in_date DATE,
			total_amount DOUBLE
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert initial data
	_, err = db.Exec(`
		INSERT INTO hotel_bookings VALUES
			(1, 'John Smith', '2025-07-01', 200.00),
			(2, 'Emma Johnson', '2025-07-02', 450.00),
			(3, 'Michael Chen', '2025-07-01', 500.00),
			(4, 'Emma Johnson', '2025-07-03', 300.00)
	`)
	if err != nil {
		t.Fatalf("Failed to insert initial data: %v", err)
	}

}
func TestDuckDb(t *testing.T) {
	setupDuckDb(t)
	sourceConfig := getDuckDbVars()
	var args []string
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	tableNameAuth := "auth_table_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	// tableNameTemplateParam := "template_param_table_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	// createParamTableStmt, insertParamTableStmt, paramToolStmt, paramToolStmt2, arrayToolStmt, paramTestParams := tests.GetDuckDbParamToolInfo()
	// createAuthTableStmt, insertAuthTableStmt, authToolStmt, authTestParams := tests.GetDuckDbAuthToolInfo(tableNameAuth)
	paramToolStmt, paramToolStmt2, arrayToolStmt, _ := tests.GetDuckDbParamToolInfo()
	authToolStmt, _ := tests.GetDuckDbAuthToolInfo(tableNameAuth)
	toolsFile := tests.GetDuckDbConfig(sourceConfig, DuckDbKind, paramToolStmt, paramToolStmt2, arrayToolStmt, authToolStmt)
	tmplSelectCombined, tmplSelectFilterCombined := tests.GetDuckDbTmplToolStatement()
	toolsFile = tests.AddTemplateParamConfig(t, toolsFile, DuckDbKind, tmplSelectCombined, tmplSelectFilterCombined, "")

	cmd, cleanup, err := tests.StartCmd(ctx, toolsFile, args...)
	if err != nil {
		t.Fatalf("command initialization returned an error: %s", err)
	}
	defer cleanup()

	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := testutils.WaitForString(waitCtx, regexp.MustCompile(`Server ready to serve`), cmd.Out)
	if err != nil {
		t.Logf("toolbox command logs: \n%s", out)
		t.Fatalf("toolbox didn't start successfully: %s", err)
	}

	tests.RunToolGetTest(t)

	select1Want := "[{\"1\":1}]"
	failInvocationWant := `{"jsonrpc":"2.0","id":"invoke-fail-tool","result":{"content":[{"type":"text","text":"unable to execute query: SQL logic error: near \"SELEC\": syntax error (1)"}],"isError":true}}`
	invokeParamWant, invokeParamWantNull, mcpInvokeParamWant := tests.GetNonSpannerInvokeParamWant()
	tests.RunToolInvokeTest(t, select1Want, invokeParamWant, invokeParamWantNull, false)
	tests.RunMCPToolCallMethod(t, mcpInvokeParamWant, failInvocationWant)
	// tests.RunToolInvokeWithTemplateParameters(t, tableNameTemplateParam, tests.NewTemplateParameterTestConfig())

}
