package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	app "github.com/etitcombe/tiddlypom"
	_ "github.com/mattn/go-sqlite3" // sqlite
)

// TiddlyStore stores data about tiddlers.
type TiddlyStore struct {
	db  *sql.DB
	dsn string
}

// NewTiddlyStore creates a new instance of a TiddlyStore.
func NewTiddlyStore(dsn string) (*TiddlyStore, error) {
	return &TiddlyStore{dsn: dsn}, nil
}

// Open opens the connection to the database.
func (ts *TiddlyStore) Open() error {
	// Ensure a DSN is set before attempting to open the database.
	if ts.dsn == "" {
		return fmt.Errorf("dsn required")
	}

	// Make the parent directory unless using an in-memory db.
	if ts.dsn != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(ts.dsn), 0700); err != nil {
			return err
		}
	}

	// Connect to the database.
	var err error
	if ts.db, err = sql.Open("sqlite3", ts.dsn); err != nil {
		return err
	}

	// Enable WAL. SQLite performs better with the WAL  because it allows
	// multiple readers to operate while data is being written.
	if _, err := ts.db.Exec(`PRAGMA journal_mode = wal;`); err != nil {
		return fmt.Errorf("enable wal: %w", err)
	}

	// Enable foreign key checks. For historical reasons, SQLite does not check
	// foreign key constraints by default... which is kinda insane. There's some
	// overhead on inserts to verify foreign key integrity but it's definitely
	// worth it.
	if _, err := ts.db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("foreign keys pragma: %w", err)
	}

	if err := ts.migrate(); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

// Close closes the connection to the data store.
func (ts *TiddlyStore) Close() error {
	if ts.db != nil {
		return ts.db.Close()
	}
	return nil
}

// Delete deletes the tiddler represented by title from the database.
func (ts *TiddlyStore) Delete(ctx context.Context, title string) error {
	tx, err := ts.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := delete(ctx, tx, title); err != nil {
		return err
	}
	return tx.Commit()
}

// Get gets a tiddler by its title.
func (ts *TiddlyStore) Get(ctx context.Context, title string) (app.Tiddler, error) {
	tx, err := ts.db.BeginTx(ctx, nil)
	if err != nil {
		return app.Tiddler{}, err
	}
	defer tx.Rollback()

	t, err := get(ctx, tx, title)
	if err != nil {
		return app.Tiddler{}, err
	}
	return t, nil
}

// GetList gets a list of all the tiddlers from the database.
func (ts *TiddlyStore) GetList(ctx context.Context) ([]app.Tiddler, error) {
	tx, err := ts.db.BeginTx(ctx, nil)
	if err != nil {
		return []app.Tiddler{}, err
	}
	defer tx.Rollback()

	t, err := getList(ctx, tx)
	if err != nil {
		return []app.Tiddler{}, err
	}
	return t, nil
}

// Upsert inserts or updates a record in the database.
func (ts *TiddlyStore) Upsert(ctx context.Context, title string, t app.Tiddler) error {
	tx, err := ts.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := upsert(ctx, tx, title, t); err != nil {
		return err
	}
	return tx.Commit()
}

func delete(ctx context.Context, tx *sql.Tx, title string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM tiddler WHERE title = ?`, title)
	if err != nil {
		return err
	}
	return nil
}

func get(ctx context.Context, tx *sql.Tx, title string) (app.Tiddler, error) {
	row := tx.QueryRowContext(ctx, "SELECT rev, meta, text FROM tiddler WHERE title = ?", title)
	if row.Err() != nil {
		return app.Tiddler{}, row.Err()
	}
	var t app.Tiddler
	err := row.Scan(&t.Rev, &t.Meta, &t.Text)
	if err != nil {
		return app.Tiddler{}, err
	}
	return t, nil
}

func getList(ctx context.Context, tx *sql.Tx) ([]app.Tiddler, error) {
	rows, err := tx.QueryContext(ctx, `SELECT meta FROM tiddler WHERE is_system = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tiddlers []app.Tiddler

	for rows.Next() {
		var t app.Tiddler
		err := rows.Scan(&t.Meta)
		if err != nil {
			return nil, err
		}
		tiddlers = append(tiddlers, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tiddlers, nil
}

func upsert(ctx context.Context, tx *sql.Tx, title string, t app.Tiddler) error {
	isSystem := 0
	if t.IsSystem {
		isSystem = 1
	}

	_, err := tx.ExecContext(ctx, `INSERT INTO tiddler
		(title, rev, meta, text, is_system)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(title) DO UPDATE SET rev = excluded.rev,
		meta = excluded.meta,
		text = excluded.text,
		is_system = excluded.is_system`, title, t.Rev, t.Meta, t.Text, isSystem)
	return err
}

// migrate sets up migration tracking and executes pending migration files.
//
// Migration files are embedded in the sqlite/migration folder and are executed
// in lexigraphical order.
//
// Once a migration is run, its name is stored in the 'migrations' table so it
// is not re-executed. Migrations run in a transaction to prevent partial
// migrations.
func (ts *TiddlyStore) migrate() error {
	// Ensure the 'migrations' table exists so we don't duplicate migrations.
	if _, err := ts.db.Exec(`CREATE TABLE IF NOT EXISTS migrations (name TEXT PRIMARY KEY);`); err != nil {
		return fmt.Errorf("cannot create migrations table: %w", err)
	}

	names, err := filepath.Glob("migration/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	// Loop over all migration files and execute them in order.
	for _, name := range names {
		if err := ts.migrateFile(name); err != nil {
			return fmt.Errorf("migration error: name=%q err=%w", name, err)
		}
	}
	return nil
}

// migrate runs a single migration file within a transaction. On success, the
// migration file name is saved to the "migrations" table to prevent re-running.
func (ts *TiddlyStore) migrateFile(name string) error {
	tx, err := ts.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Ensure migration has not already been run.
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM migrations WHERE name = ?`, name).Scan(&n); err != nil {
		return err
	} else if n != 0 {
		return nil // already run migration, skip
	}

	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read and execute migration file.
	if buf, err := ioutil.ReadAll(f); err != nil {
		return err
	} else if _, err := tx.Exec(string(buf)); err != nil {
		return err
	}

	// Insert record into migrations to prevent re-running migration.
	if _, err := tx.Exec(`INSERT INTO migrations (name) VALUES (?)`, name); err != nil {
		return err
	}

	return tx.Commit()
}
