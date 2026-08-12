// bootstrap-platform-admin creates the first platform admin account
// directly against the database, since there's no way to log in and create
// one through the API before at least one exists (the platform-admin
// endpoints are all gated by PlatformAdminMiddleware, which requires a
// valid platform-admin token).
//
// Every platform admin after the first can be created by an existing one —
// there's no API for that yet either, but this script is safe to re-run
// for additional admins in the meantime (it upserts by email).
//
// Run: go run ./scripts/bootstrap-platform-admin -apply \
//
//	-name "Praveen" -email praveen@example.com -password abcd1234
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

const platformAdminBcryptCost = 12

func main() {
	apply := flag.Bool("apply", false, "actually create the admin (dry-run otherwise)")
	name := flag.String("name", "", "platform admin's display name")
	email := flag.String("email", "", "platform admin's email (login identifier)")
	password := flag.String("password", "", "platform admin's password")
	flag.Parse()

	*name = strings.TrimSpace(*name)
	*email = strings.TrimSpace(strings.ToLower(*email))
	if *name == "" || *email == "" || *password == "" {
		log.Fatal("-name, -email, and -password are all required")
	}

	if !*apply {
		fmt.Printf("dry run: would create platform admin %q <%s>\n", *name, *email)
		fmt.Println("re-run with -apply to actually create it")
		return
	}

	config.Connect()

	if err := config.EnsureControlSchema(config.DB); err != nil {
		log.Fatalf("ensure control schema: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), platformAdminBcryptCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	admin, err := upsertPlatformAdmin(config.DB, *name, *email, string(hash))
	if err != nil {
		log.Fatalf("create platform admin: %v", err)
	}

	fmt.Printf("\nPlatform admin ready.\n")
	fmt.Printf("  id:    %s\n", admin.ID)
	fmt.Printf("  name:  %s\n", admin.Name)
	fmt.Printf("  email: %s\n", admin.Email)
	fmt.Println("  Log in with POST /api/v1/platform/login {\"email\", \"password\"}")
}

func upsertPlatformAdmin(db *gorm.DB, name, email, passwordHash string) (*models.PlatformAdmin, error) {
	var admin models.PlatformAdmin
	err := db.Where("email = ?", email).First(&admin).Error
	if err == nil {
		admin.Name = name
		admin.PasswordHash = passwordHash
		admin.IsActive = true
		if err := db.Save(&admin).Error; err != nil {
			return nil, err
		}
		return &admin, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	admin = models.PlatformAdmin{
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		IsActive:     true,
	}
	if err := db.Create(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}
