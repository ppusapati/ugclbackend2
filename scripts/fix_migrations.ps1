Set-Location "E:\Maheshwari\UGCL\backend\v1"
$path = "config\migrations.go"
$content = [System.IO.File]::ReadAllText($path)
$marker = "	})" + "`n" + "		{"
$idx = $content.IndexOf($marker)
if ($idx -lt 0) {
    # Try CRLF
    $marker = "	})" + "`r`n" + "		{"
    $idx = $content.IndexOf($marker)
}
Write-Host "Marker found at: $idx"
if ($idx -lt 0) { Write-Host "ERROR: marker not found"; exit 1 }

$goodPart = $content.Substring(0, $idx)
$newEnd = @"

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
"@
[System.IO.File]::WriteAllText($path, $goodPart + $newEnd)
Write-Host "migrations.go repaired. Length: $($goodPart.Length + $newEnd.Length)"
