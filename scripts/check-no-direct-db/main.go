// check-no-direct-db scans the codebase for direct references to the
// config.DB package-level variable outside of an explicit allowlist, and
// fails with a non-zero exit code listing every violation.
//
// Why this exists: the multi-tenancy architecture (see the schema-per-tenant
// plan) requires every request-serving code path to resolve its database
// connection through config.DBFromContext(ctx) instead of the bare config.DB
// global, so that request-scoped tenant isolation actually takes effect.
// A single missed call site doesn't fail loudly — it silently queries
// whatever schema happens to be on the connection's search_path, which is
// exactly the cross-tenant leakage risk the architecture plan calls out as
// the top risk of this whole effort. This check is the standing safeguard
// against that regression, intended to run on every future change, not just
// once during the initial rewrite.
//
// Run: go run ./scripts/check-no-direct-db
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// allowedFiles may reference config.DB directly. Each entry needs a reason:
//   - config/tenant_context.go: DBFromContext's own fallback implementation
//     — this IS the accessor, not a caller of it.
//   - config/tenant_provisioning.go, config/tenant_registry.go,
//     config/tenant_seed.go: tenant provisioning/seeding/migration code.
//     This runs as an explicit admin operation against the control schema
//     or a freshly pinned tenant connection, never as part of ordinary
//     per-request handling, so tenant context resolution doesn't apply.
//   - config/config.go, config/migrations.go, config/unified_rbac_migration.go,
//     config/seeding.go: startup/migration machinery, runs once at boot
//     against the base connection before any request exists.
//   - main.go: startup wiring and one background sync job
//     (EnsureAllActiveFormReportViews) that is intentionally global-scoped,
//     not per-tenant, as of this pass — revisit if that job needs to become
//     tenant-aware later.
//   - repos/*.go: dead code, confirmed unused by any handler (zero import
//     sites) during the RBAC migration work. Not wired into any request
//     path. Should be deleted rather than migrated if it stays unused.
//   - tmp/*.go: scratch/debug files, not part of the build or any request
//     path.
var allowedFiles = map[string]string{
	"config/tenant_context.go":         "accessor's own fallback implementation",
	"config/tenant_provisioning.go":    "tenant provisioning, not per-request",
	"config/tenant_registry.go":        "tenant registry setup, not per-request",
	"config/tenant_seed.go":            "tenant seeding, not per-request",
	"config/config.go":                 "startup connection setup",
	"config/migrations.go":             "startup migration machinery",
	"config/unified_rbac_migration.go": "one-time cutover tool",
	"config/seeding.go":                "manual/dormant seeding tool, not wired into main.go's request path",
	"main.go":                          "startup wiring + one intentionally global background job",
}

// allowedDirs may reference config.DB anywhere inside them.
var allowedDirs = []string{
	"tmp/",
	"repos/", // dead code — see allowedFiles comment above
	"scripts/",
}

var configDBPattern = regexp.MustCompile(`\bconfig\.DB\b`)

// lineSuppressionMarker allows a specific line to keep config.DB when it's
// backed by a process-wide, non-request-scoped cache (singleflight-loaded,
// keyed by an identifier that isn't a tenant), or is genuinely dead code with
// no caller. Both cases were reviewed individually during the Phase 2
// middleware rewrite — not a blanket escape hatch. Add the marker as a
// trailing comment: `config.DB.Foo() // config-db-ok: <reason>`
const lineSuppressionMarker = "config-db-ok:"

func main() {
	root := "."
	violations := []string{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		normalized := filepath.ToSlash(path)
		if strings.HasPrefix(normalized, "./") {
			normalized = normalized[2:]
		}

		if reason, ok := allowedFiles[normalized]; ok {
			_ = reason
			return nil
		}
		for _, dir := range allowedDirs {
			if strings.HasPrefix(normalized, dir) {
				return nil
			}
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if configDBPattern.MatchString(line) && !strings.Contains(line, lineSuppressionMarker) {
				violations = append(violations, fmt.Sprintf("%s:%d: %s", normalized, lineNum, trimmed))
			}
		}
		return scanner.Err()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "walk error:", err)
		os.Exit(2)
	}

	if len(violations) > 0 {
		fmt.Printf("Found %d direct config.DB reference(s) outside the allowlist.\n", len(violations))
		fmt.Println("Request-serving code must use config.DBFromContext(ctx) instead — see config/tenant_context.go.")
		fmt.Println()
		for _, v := range violations {
			fmt.Println("  " + v)
		}
		fmt.Println()
		fmt.Println("If a new file legitimately needs direct config.DB access (startup/migration/admin")
		fmt.Println("code, not per-request handling), add it to allowedFiles or allowedDirs in")
		fmt.Println("scripts/check-no-direct-db/main.go with a comment explaining why.")
		os.Exit(1)
	}

	fmt.Println("OK — no direct config.DB references found outside the allowlist.")
}
