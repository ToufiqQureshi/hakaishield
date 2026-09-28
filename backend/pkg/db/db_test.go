package db

import (
	"strings"
	"testing"
)

func TestTenantLookupQueriesSupportUUIDOwnerColumns(t *testing.T) {
	for name, query := range map[string]string{
		"host": tenantByHostQuery,
		"id":   tenantByIDQuery,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(query, "COALESCE(owner_user_id::text, '')") {
				t.Fatalf("query must cast a UUID owner to text before coalescing: %s", query)
			}
			if strings.Contains(query, "COALESCE(owner_user_id, '')") {
				t.Fatalf("query coerces an empty string to UUID and crashes on NULL owners: %s", query)
			}
		})
	}
}
