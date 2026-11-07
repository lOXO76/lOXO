package middleware

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTicketCredentialAndMessageAuditBodiesAreOmitted(t *testing.T) {
	for _, path := range []string{
		"POST /api/v1/user/tickets", "POST /api/v1/user/tickets/:id/messages", "POST /api/v1/admin/tickets/:id/messages", "PUT /api/v1/user/ticket-notify-preferences", "PUT /api/v1/admin/ticket-settings",
	} {
		_, ok := auditBodyOmittedRoutes[path]
		require.True(t, ok, path)
	}
}
