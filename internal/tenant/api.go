package tenant

import (
	"encoding/json"
	"net/http"
	"strings"
)

// APIHandler provides HTTP endpoints for tenant management.
type APIHandler struct {
	store *Store
}

// NewAPIHandler creates a tenant API handler.
func NewAPIHandler(store *Store) *APIHandler {
	return &APIHandler{store: store}
}

// ServeHTTP handles tenant API requests.
func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/tenants":
		h.list(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/tenants":
		h.create(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/tenants/"):
		h.get(w, r)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/tenants/"):
		h.update(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/tenants/"):
		h.delete(w, r)
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/suspend"):
		h.suspend(w, r)
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/activate"):
		h.activate(w, r)
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/members"):
		h.addMember(w, r)
	case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/members"):
		h.listMembers(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *APIHandler) list(w http.ResponseWriter, _ *http.Request) {
	tenants, err := h.store.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, tenants)
}

func (h *APIHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Plan string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tenant, err := h.store.Create(req.Name, req.Plan)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, tenant)
}

func (h *APIHandler) get(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/tenants/")
	if id == "" {
		http.Error(w, "missing tenant id", http.StatusBadRequest)
		return
	}

	tenant, err := h.store.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, tenant)
}

func (h *APIHandler) update(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/tenants/")
	if id == "" {
		http.Error(w, "missing tenant id", http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tenant, err := h.store.Update(id, func(t *Tenant) {
		if name, ok := updates["name"].(string); ok {
			t.Name = name
		}
		if planStr, ok := updates["plan"].(string); ok {
			t.Plan = findPlan(planStr)
			t.Quota = PlanDefaults(planStr)
		}
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, tenant)
}

func (h *APIHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/tenants/")
	if id == "" {
		http.Error(w, "missing tenant id", http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *APIHandler) suspend(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/tenants/")
	id = strings.TrimSuffix(id, "/suspend")

	tenant, err := h.store.Suspend(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, tenant)
}

func (h *APIHandler) activate(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/tenants/")
	id = strings.TrimSuffix(id, "/activate")

	tenant, err := h.store.Activate(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, tenant)
}

func (h *APIHandler) addMember(w http.ResponseWriter, r *http.Request) {
	tenantID := extractID(r.URL.Path, "/tenants/")
	tenantID = strings.TrimSuffix(tenantID, "/members")

	var req struct {
		UserID string `json:"user_id"`
		Role   Role   `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	member, err := h.store.AddMember(tenantID, req.UserID, req.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, member)
}

func (h *APIHandler) listMembers(w http.ResponseWriter, r *http.Request) {
	tenantID := extractID(r.URL.Path, "/tenants/")
	tenantID = strings.TrimSuffix(tenantID, "/members")

	members, err := h.store.ListMembers(tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, members)
}

func extractID(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.SplitN(id, "/", 2)[0]
	return id
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
