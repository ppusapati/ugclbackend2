package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type tableAudit struct {
	Count    int64  `json:"count"`
	Checksum string `json:"checksum"`
}

type rbacAudit struct {
	GeneratedAt time.Time             `json:"generated_at"`
	Tables      map[string]tableAudit `json:"tables"`
}

type auditTarget struct {
	name  string
	query string
}

func main() {
	dsnEnv := flag.String("dsn-env", "DB_DSN", "environment variable containing the PostgreSQL DSN")
	output := flag.String("output", "", "optional JSON output file")
	flag.Parse()

	_ = godotenv.Load(".env")
	dsn := os.Getenv(*dsnEnv)
	if dsn == "" {
		log.Fatalf("%s is not set", *dsnEnv)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	targets := []auditTarget{
		{
			name: "users",
			query: `SELECT id, role_id, business_vertical_id, is_active, created_at, updated_at
				FROM users ORDER BY id`,
		},
		{
			name:  "roles",
			query: `SELECT id, name, description, is_active, is_global, level, created_at, updated_at FROM roles ORDER BY id`,
		},
		{
			name:  "role_permissions",
			query: `SELECT role_id, permission_id, created_at FROM role_permissions ORDER BY role_id, permission_id`,
		},
		{
			name: "business_roles",
			query: `SELECT id, name, display_name, description, business_vertical_id, is_active, level, created_at, updated_at
				FROM business_roles ORDER BY id`,
		},
		{
			name: "business_role_permissions",
			query: `SELECT business_role_id, permission_id, created_at
				FROM business_role_permissions ORDER BY business_role_id, permission_id`,
		},
		{
			name: "user_business_roles",
			query: `SELECT id, user_id, business_role_id, is_active, assigned_at, assigned_by, created_at, updated_at
				FROM user_business_roles ORDER BY id`,
		},
		{
			name: "permissions",
			query: `SELECT id, name, description, resource, action, created_at, updated_at
				FROM permissions ORDER BY id`,
		},
	}

	audit := rbacAudit{
		GeneratedAt: time.Now().UTC(),
		Tables:      make(map[string]tableAudit, len(targets)),
	}

	for _, target := range targets {
		result, err := auditQuery(db, target.query)
		if err != nil {
			log.Fatalf("audit %s: %v", target.name, err)
		}
		audit.Tables[target.name] = result
	}

	payload, err := json.MarshalIndent(audit, "", "  ")
	if err != nil {
		log.Fatalf("marshal audit: %v", err)
	}
	payload = append(payload, '\n')

	if *output == "" {
		_, _ = os.Stdout.Write(payload)
		return
	}
	if err := os.WriteFile(*output, payload, 0o600); err != nil {
		log.Fatalf("write audit output: %v", err)
	}
	fmt.Printf("RBAC audit written to %s\n", *output)
}

func auditQuery(db *gorm.DB, orderedQuery string) (tableAudit, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) AS count,
		       COALESCE(md5(string_agg(row_to_json(audit_row)::text, '' ORDER BY row_to_json(audit_row)::text)), md5('')) AS checksum
		FROM (%s) AS audit_row`, orderedQuery)

	var result tableAudit
	if err := db.Raw(query).Scan(&result).Error; err != nil {
		return tableAudit{}, err
	}
	return result, nil
}
