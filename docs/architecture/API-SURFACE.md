# Forge API Surface

> How external tools integrate with the Forge organization.

## API Philosophy

Forge exposes ONE API surface. There are no "OpenClaw APIs" or "Suna APIs" — there is only the Forge API, which internally routes to the appropriate layer.

## The API Hierarchy

```
forge.local/api/v1/
├── /org                    # Organization-level operations
│   ├── GET    /status      # Real-time org health
│   ├── GET    /goals       # Current goals and progress
│   ├── POST   /goals       # Set a new goal
│   ├── GET    /standup     # Daily standup from all divisions
│   ├── POST   /restructure # Trigger org restructuring
│   └── GET    /costs       # Org-wide cost summary
│
├── /divisions              # Division management
│   ├── GET    /            # List all divisions
│   ├── POST   /            # Create a division
│   ├── GET    /:id         # Division status, agents, tasks
│   ├── POST   /:id/assign  # Assign work to a division
│   ├── GET    /:id/costs   # Division cost breakdown
│   └── GET    /:id/memory  # Division-specific knowledge
│
├── /agents                 # Agent management
│   ├── GET    /            # List all agents
│   ├── POST   /hire        # Create a new agent with role
│   ├── DELETE /:id         # Remove an agent
│   ├── POST   /:id/onboard # Put agent through orientation
│   ├── GET    /:id/trust   # Agent trust score
│   ├── GET    /:id/costs   # Agent cost history
│   └── GET    /:id/memory  # Agent's accumulated knowledge
│
├── /sessions               # Session management (from OpenClaw)
│   ├── GET    /            # List sessions
│   ├── POST   /            # Create session
│   ├── GET    /:id         # Session details
│   ├── POST   /:id/send    # Send message, get response
│   ├── POST   /:id/branch  # Branch a session
│   └── PATCH  /:id         # Update session (model, tags)
│
├── /tasks                  # Task management
│   ├── GET    /            # List tasks (filterable)
│   ├── POST   /            # Create a task
│   ├── GET    /:id         # Task details
│   ├── PATCH  /:id         # Update task status
│   └── POST   /:id/approve # Approve task result
│
├── /memory                 # Knowledge base
│   ├── GET    /search      # Semantic search
│   ├── POST   /            # Store knowledge
│   ├── GET    /:key        # Retrieve by key
│   └── DELETE /:key        # Remove entry
│
├── /skills                 # Skill management
│   ├── GET    /            # List installed skills
│   ├── GET    /marketplace # Browse marketplace (from Suna)
│   ├── POST   /install     # Install a skill
│   └── POST   /invoke      # Execute a skill
│
├── /channels               # Communication channels
│   ├── GET    /            # List channels
│   ├── POST   /:id/send    # Send message to channel
│   └── GET    /:id/history # Channel message history
│
├── /costs                  # Cost tracking
│   ├── GET    /live        # Real-time cost stream (WebSocket)
│   ├── GET    /summary     # Period summary
│   ├── POST   /budget      # Set budget limits
│   └── GET    /ledger      # Immutable cost ledger
│
├── /compliance             # Compliance & audit
│   ├── GET    /status      # Compliance posture
│   ├── GET    /audit       # Audit trail
│   └── POST   /report      # Generate compliance report
│
└── /integrations           # External integrations
    ├── GET    /            # List connected services
    ├── POST   /connect     # Connect a service
    └── DELETE /:id         # Disconnect a service
```

## Authentication

All API calls use the same auth token:

```bash
# From CLI
forge api get /org/status

# From external tool
curl -H "Authorization: Bearer $FORGE_TOKEN" http://forge.local/api/v1/org/status
```

## External Tool Integration Patterns

### Pattern 1: REST Consumer (any language)
```python
import requests
forge = requests.Session()
forge.headers["Authorization"] = f"Bearer {os.environ['FORGE_TOKEN']}"

# Assign work to engineering division
forge.post("http://forge.local/api/v1/divisions/eng/assign", json={
    "task": "Implement user authentication",
    "priority": "high",
    "deadline": "2026-06-01"
})
```

### Pattern 2: MCP Client (Claude, Cursor, Copilot)
```
forge mcp serve  # Exposes all Forge tools via MCP protocol
```

External tools see Forge as an MCP server with tools like:
- `forge_division_assign` — assign work to a division
- `forge_agent_hire` — create a new agent
- `forge_memory_search` — search org knowledge
- `forge_cost_check` — check remaining budget

### Pattern 3: Webhook Receiver
```
forge bridge serve  # Starts bridge with HTTP endpoint
POST /webhooks/github → Forge creates task in engineering division
POST /webhooks/stripe → Forge updates finance division
```

### Pattern 4: CLI Embedding
```bash
# From any CI/CD pipeline
forge task create --division=engineering --priority=high "Fix auth bug #123"
forge cost check --division=engineering
forge memory search "how did we fix the auth issue last time?"
```

## API Design Rules

1. **Org vocabulary everywhere.** No "sessions" without an agent. No "tasks" without a division. Everything is scoped to the org.
2. **Consistent error format.** `{"error": "code", "message": "human-readable", "fix": "suggested action"}`.
3. **Cost on every response.** Every API response includes `cost_usd` and `tokens_used` headers.
4. **Audit by default.** Every mutation is logged to the immutable audit trail.
5. **Idempotent writes.** POST with same idempotency-key returns cached result.
