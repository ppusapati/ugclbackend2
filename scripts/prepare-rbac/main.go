package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
)

type migrationAuditTable struct {
	Count    int64  `json:"count"`
	Checksum string `json:"checksum"`
}

type migrationAudit struct {
	Tables map[string]migrationAuditTable `json:"tables"`
}

func main() {
	apply := flag.Bool("apply", false, "apply the additive unified RBAC copy")
	backupPath := flag.String("backup", "", "verified custom-format pg_dump file (mutually exclusive with -cloud-backup-id)")
	checksumPath := flag.String("checksum", "", "SHA-256 file produced by backup verification (required with -backup)")
	cloudBackupID := flag.String("cloud-backup-id", "", "Cloud SQL backup ID verified via native snapshot+clone (mutually exclusive with -backup)")
	sourceAuditPath := flag.String("source-audit", "", "source RBAC audit produced during backup")
	restoreAuditPath := flag.String("restore-audit", "", "restored/clone database RBAC audit")
	flag.Parse()

	if !*apply {
		log.Fatal("refusing to run without -apply")
	}

	useCloudBackup := strings.TrimSpace(*cloudBackupID) != ""
	useLocalBackup := strings.TrimSpace(*backupPath) != ""
	if useCloudBackup && useLocalBackup {
		log.Fatal("-cloud-backup-id and -backup are mutually exclusive")
	}
	if !useCloudBackup && !useLocalBackup {
		log.Fatal("one of -backup or -cloud-backup-id is required")
	}

	for name, path := range map[string]string{
		"source audit":  *sourceAuditPath,
		"restore audit": *restoreAuditPath,
	} {
		if strings.TrimSpace(path) == "" {
			log.Fatalf("%s path is required", name)
		}
		if _, err := os.Stat(path); err != nil {
			log.Fatalf("%s is unavailable: %v", name, err)
		}
	}

	if useLocalBackup {
		if strings.TrimSpace(*checksumPath) == "" {
			log.Fatal("-checksum is required with -backup")
		}
		if _, err := os.Stat(*checksumPath); err != nil {
			log.Fatalf("checksum file is unavailable: %v", err)
		}
		if err := verifyBackupChecksum(*backupPath, *checksumPath); err != nil {
			log.Fatalf("backup verification failed: %v", err)
		}
	} else {
		log.Printf("Cloud SQL backup ID %s accepted; skipping local dump checksum", *cloudBackupID)
	}

	sourceAudit, err := loadMigrationAudit(*sourceAuditPath)
	if err != nil {
		log.Fatalf("load source audit: %v", err)
	}
	restoreAudit, err := loadMigrationAudit(*restoreAuditPath)
	if err != nil {
		log.Fatalf("load restore audit: %v", err)
	}
	if err := compareMigrationAudits(sourceAudit, restoreAudit); err != nil {
		log.Fatalf("restore audit does not match source: %v", err)
	}

	_ = godotenv.Load(".env")
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN is not set")
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		log.Fatalf("get working directory: %v", err)
	}
	liveAuditPath := filepath.Join(os.TempDir(), "ugcl-rbac-live-"+filepath.Base(*backupPath)+".json")
	defer os.Remove(liveAuditPath)
	if err := runLiveAudit(workingDirectory, liveAuditPath); err != nil {
		log.Fatalf("run live pre-migration audit: %v", err)
	}
	liveAudit, err := loadMigrationAudit(liveAuditPath)
	if err != nil {
		log.Fatalf("load live audit: %v", err)
	}
	if err := compareMigrationAudits(sourceAudit, liveAudit); err != nil {
		log.Fatalf("database changed after backup; take and verify a new backup: %v", err)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	if err := config.PrepareRBACMigration(db); err != nil {
		log.Fatalf("unified RBAC copy rolled back: %v", err)
	}

	postAuditPath := filepath.Join(os.TempDir(), "ugcl-rbac-post-"+filepath.Base(*backupPath)+".json")
	defer os.Remove(postAuditPath)
	if err := runLiveAudit(workingDirectory, postAuditPath); err != nil {
		log.Fatalf("run live post-migration audit: %v", err)
	}
	postAudit, err := loadMigrationAudit(postAuditPath)
	if err != nil {
		log.Fatalf("load post-migration audit: %v", err)
	}
	if err := compareMigrationAudits(sourceAudit, postAudit); err != nil {
		log.Fatalf("legacy RBAC data changed unexpectedly: %v", err)
	}

	fmt.Println("Unified RBAC tables prepared successfully; legacy RBAC checksums are unchanged")
}

func verifyBackupChecksum(backupPath, checksumPath string) error {
	checksumFile, err := os.Open(checksumPath)
	if err != nil {
		return err
	}
	defer checksumFile.Close()

	scanner := bufio.NewScanner(checksumFile)
	if !scanner.Scan() {
		return fmt.Errorf("checksum file is empty")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) == 0 {
		return fmt.Errorf("checksum file is invalid")
	}
	expected := strings.ToLower(fields[0])

	backup, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer backup.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, backup); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("SHA-256 mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

func loadMigrationAudit(path string) (migrationAudit, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return migrationAudit{}, err
	}
	var audit migrationAudit
	if err := json.Unmarshal(data, &audit); err != nil {
		return migrationAudit{}, err
	}
	if len(audit.Tables) == 0 {
		return migrationAudit{}, fmt.Errorf("audit has no tables")
	}
	return audit, nil
}

func compareMigrationAudits(expected, actual migrationAudit) error {
	if len(expected.Tables) != len(actual.Tables) {
		return fmt.Errorf("table count differs: got %d, want %d", len(actual.Tables), len(expected.Tables))
	}
	for name, expectedTable := range expected.Tables {
		actualTable, ok := actual.Tables[name]
		if !ok {
			return fmt.Errorf("table %s is missing", name)
		}
		if actualTable.Count != expectedTable.Count || actualTable.Checksum != expectedTable.Checksum {
			return fmt.Errorf(
				"table %s differs: got count=%d checksum=%s, want count=%d checksum=%s",
				name,
				actualTable.Count,
				actualTable.Checksum,
				expectedTable.Count,
				expectedTable.Checksum,
			)
		}
	}
	return nil
}

func runLiveAudit(workingDirectory, outputPath string) error {
	command := exec.Command("go", "run", "./scripts/rbac-audit", "-output", outputPath)
	command.Dir = workingDirectory
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
