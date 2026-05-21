package integration

import "context"

// ProviderClient is the interface each integration backend must implement.
// All methods receive a context for cancellation and timeouts.
type ProviderClient interface {
	// TestConnectivity checks that credentials work.
	TestConnectivity(ctx context.Context) error

	// FetchTasks retrieves tasks. Supports optional filters.
	FetchTasks(ctx context.Context, conn *Connection, filters TaskFilters) ([]*Task, error)

	// CreateTask creates a task in the remote provider and returns the
	// provider-assigned key (e.g., "PROJ-123").
	CreateTask(ctx context.Context, conn *Connection, task *Task) (providerKey string, err error)

	// UpdateTask modifies a task in the remote provider.
	UpdateTask(ctx context.Context, conn *Connection, task *Task) error

	// AddComment adds a comment to a task.
	AddComment(ctx context.Context, conn *Connection, taskID string, author string, body string) error

	// GetTask retrieves a single task by provider key.
	GetTask(ctx context.Context, conn *Connection, key string) (*Task, error)
}

// TaskFilters holds optional query parameters for FetchTasks.
type TaskFilters struct {
	Status   string   // filter by status name
	Assignee string   // filter by assignee
	Labels   []string // filter by labels
	Limit    int      // max results (0 = provider default)
	Project  string   // filter by project
}

// providerRegistry maps provider names to factory functions.
var providerRegistry = map[Provider]func() ProviderClient{
	ProviderJira:   func() ProviderClient { return &JiraClient{} },
	ProviderLinear: func() ProviderClient { return &LinearClient{} },
	ProviderNotion: func() ProviderClient { return &NotionClient{} },
}

// NewProviderClient returns a client for the given provider.
func NewProviderClient(p Provider) ProviderClient {
	fn, ok := providerRegistry[p]
	if !ok {
		return nil
	}
	return fn()
}
