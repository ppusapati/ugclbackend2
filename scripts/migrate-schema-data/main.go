// migrate-schema-data moves every application table's data out of the
// public schema and into a named tenant schema, using ALTER TABLE ... SET
// SCHEMA — a metadata-only catalog change, not a row-by-row copy, so it's
// fast and atomic regardless of table size. This is the "Phase 6 migration"
// referenced in config/tenant_provisioning.go's TenantScopedSession doc
// comment: existing pre-multi-tenancy data lived in public; each real
// tenant needs it moved (not copied) into that tenant's own schema so
// public no longer holds any tenant application data.
//
// Extension-owned tables (spatial_ref_sys, and anything else PostGIS/
// PostGIS-topology creates) are auto-detected via pg_depend and always
// excluded — they belong to the extension, not the application, and must
// stay reachable via every tenant's search_path fallback to public.
//
// Safety:
//   - Refuses to run without -apply (dry-run lists what would move).
//   - Requires the destination schema to already exist and already have
//     the full migrated table set (i.e. run ProvisionTenantSchema /
//     bootstrap-tenant first) — this script does not create schemas or run
//     migrations, it only moves data.
//   - If the destination schema already has a same-named table (e.g. from
//     ProvisionTenantSchema's migration run, or SeedNewTenantRBAC's initial
//     admin), that table is DROPPED (CASCADE) before the source table takes
//     its place. This is intentional — the destination's freshly-migrated
//     empty/seed tables are meant to be replaced by the real data being
//     moved in, not merged with it. Re-seed the destination's initial admin
//     user (SeedNewTenantRBAC / bootstrap-tenant) after this runs.
//   - Every DROP and ALTER TABLE runs inside one transaction: either
//     everything moves or nothing does.
//   - Verifies row counts in the destination schema match the source counts
//     captured before the move, inside the same transaction, before commit.
//   - Take a verified backup (e.g. `gcloud sql backups create`) before
//     running this with -apply. This tool does not take one for you.
//
// Run:
//
//	go run ./scripts/migrate-schema-data -to sreeugcl            # dry run
//	go run ./scripts/migrate-schema-data -to sreeugcl -apply     # for real
package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"

	"gorm.io/gorm"
	"p9e.in/ugcl/config"
)

func main() {
	toSchema := flag.String("to", "", "destination tenant schema name (must already exist and be migrated)")
	apply := flag.Bool("apply", false, "actually move the tables (dry-run otherwise)")
	flag.Parse()

	*toSchema = strings.TrimSpace(*toSchema)
	if *toSchema == "" {
		log.Fatal("-to is required")
	}
	if err := config.ValidateTenantSchemaName(*toSchema); err != nil {
		log.Fatalf("invalid destination schema %q: %v", *toSchema, err)
	}
	if *toSchema == "public" {
		log.Fatal("destination schema cannot be public")
	}

	config.Connect()

	if err := requireSchemaExists(config.DB, *toSchema); err != nil {
		log.Fatalf("destination schema check failed: %v", err)
	}

	tables, err := listMovableTables(config.DB)
	if err != nil {
		log.Fatalf("list movable tables: %v", err)
	}
	if len(tables) == 0 {
		log.Fatal("no movable tables found in public — already migrated?")
	}

	sourceCounts, err := countRows(config.DB, "public", tables)
	if err != nil {
		log.Fatalf("count source rows: %v", err)
	}

	sort.Strings(tables)
	totalRows := int64(0)
	fmt.Printf("Tables to move from public -> %s:\n", *toSchema)
	for _, t := range tables {
		fmt.Printf("  %-40s %d rows\n", t, sourceCounts[t])
		totalRows += sourceCounts[t]
	}
	fmt.Printf("\n%d tables, %d total rows\n", len(tables), totalRows)

	if !*apply {
		fmt.Println("\ndry run only — re-run with -apply to actually move the data")
		return
	}

	fmt.Printf("\nmoving %d tables into %s ...\n", len(tables), *toSchema)
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		// The destination schema was already migrated (bootstrap-tenant /
		// ProvisionTenantSchema), so it has its own empty (or seed-data)
		// copy of every table with the same name. ALTER TABLE ... SET SCHEMA
		// fails on a name collision, so drop the destination's copy first —
		// the source table (with the real data) takes its place. This
		// intentionally discards whatever seed data the destination schema
		// had (e.g. an initial admin user); re-seed after this script runs.
		for _, t := range tables {
			var destExists bool
			existsStmt := `SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = ? AND table_name = ?)`
			if err := tx.Raw(existsStmt, *toSchema, t).Scan(&destExists).Error; err != nil {
				return fmt.Errorf("check destination table %s: %w", t, err)
			}
			if destExists {
				dropStmt := fmt.Sprintf("DROP TABLE %s.%s CASCADE", quoteIdent(*toSchema), quoteIdent(t))
				if err := tx.Exec(dropStmt).Error; err != nil {
					return fmt.Errorf("drop destination table %s.%s: %w", *toSchema, t, err)
				}
			}
		}

		for _, t := range tables {
			stmt := fmt.Sprintf("ALTER TABLE public.%s SET SCHEMA %s", quoteIdent(t), *toSchema)
			if err := tx.Exec(stmt).Error; err != nil {
				return fmt.Errorf("move table %s: %w", t, err)
			}
		}

		destCounts, err := countRows(tx, *toSchema, tables)
		if err != nil {
			return fmt.Errorf("count destination rows: %w", err)
		}
		for _, t := range tables {
			if destCounts[t] != sourceCounts[t] {
				return fmt.Errorf("row count mismatch for %s: source had %d, destination has %d", t, sourceCounts[t], destCounts[t])
			}
		}

		var remaining int64
		if err := tx.Raw(`
			SELECT count(*) FROM information_schema.tables t
			WHERE t.table_schema = 'public' AND t.table_type = 'BASE TABLE'
			  AND t.table_name != 'spatial_ref_sys'
		`).Scan(&remaining).Error; err != nil {
			return fmt.Errorf("verify public is empty: %w", err)
		}
		if remaining != 0 {
			return fmt.Errorf("public still has %d non-extension tables after move — refusing to commit", remaining)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("migration failed and was rolled back: %v", err)
	}

	fmt.Printf("\ndone: %d tables (%d rows) moved from public into %s\n", len(tables), totalRows, *toSchema)
	fmt.Println("public now holds only extension-owned tables (spatial_ref_sys, etc.)")
}

// requireSchemaExists fails fast if the destination schema hasn't been
// provisioned yet, rather than silently creating it — creating a schema is
// bootstrap-tenant's job, not this tool's.
func requireSchemaExists(db *gorm.DB, schema string) error {
	var exists bool
	if err := db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = ?)`, schema).
		Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("schema %q does not exist — run bootstrap-tenant first", schema)
	}
	return nil
}

// listMovableTables returns every base table in public except ones owned
// by an extension (PostGIS, PostGIS-topology, etc.) — those belong to the
// extension, not the application, and must never move.
func listMovableTables(db *gorm.DB) ([]string, error) {
	var extOwned []string
	if err := db.Raw(`
		SELECT c.relname
		FROM pg_depend d
		JOIN pg_extension e ON d.refobjid = e.oid
		JOIN pg_class c ON d.objid = c.oid
		JOIN pg_namespace n ON c.relnamespace = n.oid
		WHERE d.deptype = 'e' AND n.nspname = 'public' AND c.relkind = 'r'
	`).Scan(&extOwned).Error; err != nil {
		return nil, fmt.Errorf("list extension-owned tables: %w", err)
	}
	excluded := make(map[string]bool, len(extOwned))
	for _, t := range extOwned {
		excluded[t] = true
	}

	var all []string
	if err := db.Raw(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
	`).Scan(&all).Error; err != nil {
		return nil, fmt.Errorf("list public tables: %w", err)
	}

	movable := make([]string, 0, len(all))
	for _, t := range all {
		if !excluded[t] {
			movable = append(movable, t)
		}
	}
	return movable, nil
}

func countRows(db *gorm.DB, schema string, tables []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(tables))
	for _, t := range tables {
		var count int64
		stmt := fmt.Sprintf("SELECT count(*) FROM %s.%s", quoteIdent(schema), quoteIdent(t))
		if err := db.Raw(stmt).Scan(&count).Error; err != nil {
			return nil, fmt.Errorf("count %s.%s: %w", schema, t, err)
		}
		counts[t] = count
	}
	return counts, nil
}

func quoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
