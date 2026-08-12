// bootstrap-tenant creates one tenant end-to-end: a control.tenants row,
// its Postgres schema (fully migrated), and a seeded super_admin user able
// to log in immediately. This is the first real caller of
// config.ProvisionTenantSchema and config.SeedNewTenantRBAC, which existed
// but had no caller until now.
//
// Run: go run ./scripts/bootstrap-tenant -apply \
//
//	-name "Acme Construction" -slug acme \
//	-admin-name "Praveen" -admin-phone 8465000099 -admin-password abcd1234 \
//	[-admin-email praveen@example.com]
//
// Idempotent: re-running with the same -slug reuses the existing tenant row
// and re-provisions/re-seeds safely (ProvisionTenantSchema and
// SeedNewTenantRBAC are both written to be retry-safe).
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

func main() {
	apply := flag.Bool("apply", false, "actually create the tenant (dry-run otherwise)")
	name := flag.String("name", "", "tenant display name")
	slug := flag.String("slug", "", "tenant slug, used for login's tenant_slug field")
	schemaName := flag.String("schema", "", "Postgres schema name (defaults to slug with '-' replaced by '_')")
	adminName := flag.String("admin-name", "", "initial super_admin user's display name")
	adminEmail := flag.String("admin-email", "", "initial super_admin user's email (optional)")
	adminPhone := flag.String("admin-phone", "", "initial super_admin user's phone (login identifier)")
	adminPassword := flag.String("admin-password", "", "initial super_admin user's password")
	flag.Parse()

	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*slug) == "" {
		log.Fatal("-name and -slug are required")
	}
	if strings.TrimSpace(*adminPhone) == "" || strings.TrimSpace(*adminPassword) == "" {
		log.Fatal("-admin-phone and -admin-password are required")
	}
	resolvedSchema := strings.TrimSpace(*schemaName)
	if resolvedSchema == "" {
		resolvedSchema = strings.ReplaceAll(strings.TrimSpace(*slug), "-", "_")
	}
	if err := config.ValidateTenantSchemaName(resolvedSchema); err != nil {
		log.Fatalf("invalid schema name %q: %v", resolvedSchema, err)
	}

	if !*apply {
		fmt.Printf("dry run: would create tenant %q (slug=%q, schema=%q) with super_admin phone=%q\n",
			*name, *slug, resolvedSchema, *adminPhone)
		fmt.Println("re-run with -apply to actually create it")
		return
	}

	config.Connect()

	if err := config.EnsureControlSchema(config.DB); err != nil {
		log.Fatalf("ensure control schema: %v", err)
	}

	tenant, err := upsertTenant(config.DB, *name, *slug, resolvedSchema)
	if err != nil {
		log.Fatalf("create tenant row: %v", err)
	}
	log.Printf("tenant row ready: id=%s slug=%s schema=%s", tenant.ID, tenant.Slug, tenant.SchemaName)

	if err := config.ProvisionTenantSchema(tenant.SchemaName); err != nil {
		log.Fatalf("provision tenant schema: %v", err)
	}
	log.Printf("tenant schema %s provisioned and migrated", tenant.SchemaName)

	tenantDB, cleanup, err := config.TenantScopedSession(tenant.SchemaName)
	if err != nil {
		log.Fatalf("open tenant session: %v", err)
	}
	defer cleanup()

	seedInput := config.SeedNewTenantRBACInput{
		SuperAdminName:     strings.TrimSpace(*adminName),
		SuperAdminEmail:    strings.TrimSpace(*adminEmail),
		SuperAdminPhone:    strings.TrimSpace(*adminPhone),
		SuperAdminPassword: *adminPassword,
	}
	if err := config.SeedNewTenantRBAC(tenantDB, seedInput); err != nil {
		log.Fatalf("seed super admin: %v", err)
	}

	if err := markTenantActive(config.DB, tenant); err != nil {
		log.Fatalf("mark tenant active: %v", err)
	}

	fmt.Printf("\nTenant %q ready.\n", *name)
	fmt.Printf("  tenant_slug: %s\n", tenant.Slug)
	fmt.Printf("  login phone: %s\n", *adminPhone)
	fmt.Println("  Log in with POST /api/v1/login {\"tenant_slug\", \"phone\", \"password\"}")
}

// upsertTenant creates the control.tenants row if it doesn't already exist
// for this slug, so re-running the script is safe.
func upsertTenant(db *gorm.DB, name, slug, schemaName string) (*models.Tenant, error) {
	var tenant models.Tenant
	err := db.Where("slug = ?", slug).First(&tenant).Error
	if err == nil {
		if tenant.SchemaName != schemaName {
			return nil, fmt.Errorf("tenant slug %q already exists with schema %q, not %q", slug, tenant.SchemaName, schemaName)
		}
		return &tenant, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	tenant = models.Tenant{
		Name:       name,
		Slug:       slug,
		SchemaName: schemaName,
		Status:     models.TenantStatusProvisioning,
		IsActive:   true,
	}
	if err := db.Create(&tenant).Error; err != nil {
		return nil, err
	}
	return &tenant, nil
}

func markTenantActive(db *gorm.DB, tenant *models.Tenant) error {
	return db.Model(tenant).Updates(map[string]interface{}{
		"status": models.TenantStatusActive,
	}).Error
}
