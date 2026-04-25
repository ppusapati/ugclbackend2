//go:build ignore

package main

import (
	"log"
	"os"
	"strings"
)

func main() {
	path := `E:\Maheshwari\UGCL\backend\v1\config\migrations.go`
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	content := string(raw)

	// Find the corruption boundary: `})` immediately followed by newline + `		{`
	marker := "\t})\n\t\t{"
	idx := strings.Index(content, marker)
	if idx < 0 {
		log.Fatal("corruption marker not found")
	}

	goodPart := content[:idx]
	newEnd := `
		{
			ID: "20260425_notifications_add_conversation_message_id",
			Migrate: func(tx *gorm.DB) error {
				queries := []string{
					"ALTER TABLE notifications ADD COLUMN IF NOT EXISTS conversation_id UUID",
					"ALTER TABLE notifications ADD COLUMN IF NOT EXISTS message_id UUID",
					"CREATE INDEX IF NOT EXISTS idx_notifications_conversation_id ON notifications(conversation_id)",
					"CREATE INDEX IF NOT EXISTS idx_notifications_message_id ON notifications(message_id)",
				}
				for _, q := range queries {
					if err := tx.Exec(q).Error; err != nil {
						return err
					}
				}
				return nil
			},
		},
	})

	return m.Migrate()
}
`
	if err := os.WriteFile(path, []byte(goodPart+newEnd), 0644); err != nil {
		log.Fatal(err)
	}
	log.Println("migrations.go repaired")
}
