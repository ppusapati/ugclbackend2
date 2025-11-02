package utils

import "testing"

func TestMatchesPermission(t *testing.T) {
	tests := []struct {
		name         string
		userPerm     string
		requiredPerm string
		expected     bool
	}{
		// Exact matches
		{"exact match same permission", "project:create", "project:create", true},
		{"exact match different permission", "project:create", "project:read", false},
		{"exact match different resource", "project:create", "user:create", false},

		// Full wildcard tests
		{"full wildcard *:*:*", "*:*:*", "project:create", true},
		{"full wildcard *", "*", "anything:goes", true},
		{"full wildcard matches all resources", "*:*:*", "user:delete", true},
		{"full wildcard matches all actions", "*:*:*", "report:export", true},

		// Resource wildcard tests
		{"resource wildcard matches create", "project:*", "project:create", true},
		{"resource wildcard matches read", "project:*", "project:read", true},
		{"resource wildcard matches update", "project:*", "project:update", true},
		{"resource wildcard matches delete", "project:*", "project:delete", true},
		{"resource wildcard doesn't match different resource", "project:*", "user:create", false},

		// Action wildcard tests
		{"action wildcard matches project", "*:read", "project:read", true},
		{"action wildcard matches user", "*:read", "user:read", true},
		{"action wildcard matches report", "*:read", "report:read", true},
		{"action wildcard doesn't match different action", "*:read", "project:create", false},
		{"action wildcard doesn't match write", "*:read", "user:write", false},

		// Complex patterns
		{"wildcard both ways resource", "project:*", "project:approve", true},
		{"wildcard both ways action", "*:delete", "contract:delete", true},

		// Old format backward compatibility
		{"old format exact match", "read_reports", "read_reports", true},
		{"old format no match", "read_reports", "create_reports", false},
		{"old format with wildcard no match", "*:*:*", "old_format_perm", true},

		// Edge cases
		{"empty required permission", "project:create", "", false},
		{"empty user permission", "", "project:create", false},
		{"both empty", "", "", true},
		{"single part permission", "admin", "admin", true},
		{"single part vs multi-part", "admin", "admin:read", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesPermission(tt.userPerm, tt.requiredPerm)
			if result != tt.expected {
				t.Errorf("MatchesPermission(%q, %q) = %v, expected %v",
					tt.userPerm, tt.requiredPerm, result, tt.expected)
			}
		})
	}
}

func TestMatchesPermission_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name      string
		userRole  string
		userPerms []string
		required  string
		expected  bool
	}{
		{
			name:      "super admin has all access",
			userRole:  "super_admin",
			userPerms: []string{"*:*:*"},
			required:  "project:delete",
			expected:  true,
		},
		{
			name:      "project manager has all project permissions",
			userRole:  "project_manager",
			userPerms: []string{"project:*"},
			required:  "project:approve",
			expected:  true,
		},
		{
			name:      "project manager cannot manage users",
			userRole:  "project_manager",
			userPerms: []string{"project:*"},
			required:  "user:create",
			expected:  false,
		},
		{
			name:      "analyst has read-only access",
			userRole:  "analyst",
			userPerms: []string{"*:read"},
			required:  "report:read",
			expected:  true,
		},
		{
			name:      "analyst cannot create",
			userRole:  "analyst",
			userPerms: []string{"*:read"},
			required:  "report:create",
			expected:  false,
		},
		{
			name:      "specific permission only",
			userRole:  "report_creator",
			userPerms: []string{"report:create"},
			required:  "report:create",
			expected:  true,
		},
		{
			name:      "specific permission denied for different action",
			userRole:  "report_creator",
			userPerms: []string{"report:create"},
			required:  "report:delete",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate checking if user has required permission
			hasPermission := false
			for _, userPerm := range tt.userPerms {
				if MatchesPermission(userPerm, tt.required) {
					hasPermission = true
					break
				}
			}

			if hasPermission != tt.expected {
				t.Errorf("User with role %q and permissions %v: expected %v for %q, got %v",
					tt.userRole, tt.userPerms, tt.expected, tt.required, hasPermission)
			}
		})
	}
}

func BenchmarkMatchesPermission_ExactMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MatchesPermission("project:create", "project:create")
	}
}

func BenchmarkMatchesPermission_WildcardMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MatchesPermission("*:*:*", "project:create")
	}
}

func BenchmarkMatchesPermission_ResourceWildcard(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MatchesPermission("project:*", "project:create")
	}
}

func BenchmarkMatchesPermission_ActionWildcard(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MatchesPermission("*:read", "project:read")
	}
}

func BenchmarkMatchesPermission_NoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MatchesPermission("project:create", "user:delete")
	}
}
