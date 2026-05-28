// Command dbcheck is a tiny fallback used by wrap_test.sh when the sqlite3
// CLI isn't installed on the host. It prints "<sessions> <tool_calls>"
// counted from the given database file.
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dbcheck <db-path>")
		os.Exit(2)
	}
	db, err := sql.Open("sqlite", "file:"+os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer db.Close()
	var sessions, calls int
	if err := db.QueryRow(`SELECT count(*) FROM sessions`).Scan(&sessions); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := db.QueryRow(
		`SELECT count(*) FROM tool_calls WHERE direction='outbound' AND tool_name LIKE 'tools/call:%'`,
	).Scan(&calls); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("%d %d\n", sessions, calls)
}
