package tenant

import (
	"context"
	"net/http"
	"strings"
)

// Middleware provides tenant-aware HTTP middleware.
type Middleware struct {
	store *Store
}

// NewMiddleware creates tenant middleware.
func NewMiddleware(store *Store) *Middleware {
	return &Middleware{store: store}
}

// RequireTenant extracts the tenant ID from the request and validates it.
func (m *Middleware) RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractTenantFromHeader(r)
		if tenantID == "" {
			tenantID = extractTenantFromQuery(r)
		}

		if tenantID == "" {
			http.Error(w, "missing tenant id (X-Tenant-ID header or tenant query param)", http.StatusUnauthorized)
			return
		}

		tenant, err := m.store.Get(tenantID)
		if err != nil {
			http.Error(w, "invalid tenant", http.StatusForbidden)
			return
		}

		if tenant.Status != "active" {
			http.Error(w, "tenant is not active", http.StatusForbidden)
			return
		}

		// Add tenant to context
		ctx := context.WithValue(r.Context(), tenantCtxKey{}, tenant)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole checks if the authenticated user has the required role.
func (m *Middleware) RequireRole(minRole Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roleStr := r.Header.Get("X-User-Role")
		if roleStr == "" {
			roleStr = "viewer"
		}

		role := Role(roleStr)
		if !roleAtLeast(role, minRole) {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// QuotaMiddleware checks tenant quota before allowing requests.
func (m *Middleware) QuotaMiddleware(currentAgents, currentSessions int, currentCostUSD float64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := TenantFromContext(r.Context())
		if tenant == nil {
			http.Error(w, "tenant not found in context", http.StatusUnauthorized)
			return
		}

		if err := CheckQuota(tenant, currentAgents, currentSessions, currentCostUSD); err != nil {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// TenantFromContext retrieves the tenant from the request context.
func TenantFromContext(ctx context.Context) *Tenant {
	tenant, ok := ctx.Value(tenantCtxKey{}).(*Tenant)
	if !ok {
		return nil
	}
	return tenant
}

type tenantCtxKey struct{}

func extractTenantFromHeader(r *http.Request) string {
	return r.Header.Get("X-Tenant-ID")
}

func extractTenantFromQuery(r *http.Request) string {
	return r.URL.Query().Get("tenant")
}

// roleAtLeast checks if a role meets the minimum requirement.
func roleAtLeast(role, minRole Role) bool {
	roleLevel := map[Role]int{
		RoleOwner:  4,
		RoleAdmin:  3,
		RoleMember: 2,
		RoleViewer: 1,
	}

	level, ok := roleLevel[role]
	if !ok {
		return false
	}
	minLevel, ok := roleLevel[minRole]
	if !ok {
		return false
	}

	return level >= minLevel
}

// Ensure unused import is avoided
var _ = strings.TrimSpace
