# 013 — Multi-Resolution Communication

> Gap: #22 Stakeholder Communication

## Problem Statement

"Give me a status update" gets either silence or a wall of logs. No middle ground. A CEO needs one paragraph. An engineer needs the stack trace. A CFO needs the cost impact. Same data, three views. Agents communicate in raw output. Forge generates resolution-appropriate views from any data.

## Design Decisions

### Why View Generation, Not Summarization

Summarization loses information. View generation preserves it at different granularities. The same data block produces:

- **Executive**: "3 features shipped. 2 bugs found. Cost: $47. On track for Friday."
- **Technical**: "Feature A: 12 files changed, 847 insertions. Bug B: auth.go line 42 nil dereference. Cost breakdown: GPT-4 $32, Claude $15."
- **Financial**: "Revenue impact: +$2,400/month (new feature). Cost: $47 this sprint. ROI: 51:1. Burn rate: trending down 12%."
- **Operational**: "Deploy at 14:00. Estimated downtime: 0. Rollback plan: git revert. Monitoring: enhanced for 24h."

### Audience Profiles

The system knows its audience:
```go
type Audience struct {
    Role        string   // "ceo", "engineer", "cfo", "lawyer"
    Expertise   []string // domains they understand
    Interests   []string // what they care about
    DetailLevel int      // 1-5 (summary to raw data)
    Format      string   // "text", "table", "chart", "slides"
}
```

### Format Adaptation

Same data, different formats:
- **Text**: Natural language paragraph
- **Table**: Structured data in rows/columns
- **Chart**: Visual representation (rendered for UI)
- **Slides**: Presentation-ready bullet points
- **Raw**: Original data for programmatic consumption

### Progressive Disclosure

Start with the executive summary. Each level reveals more detail:
```
Level 1: "We're on track." 
Level 2: "3/5 features done, 2 in progress, 0 blocked."
Level 3: "Feature A: done. Feature B: 80% complete. Feature C: blocked by auth bug."
Level 4: [Full task list with assignees, PRs, commit history, test results]
Level 5: [Raw JSON data]
```

## API Surface

```go
type MultiResolution struct { ... }

// Generate a view of data at a specific resolution
func (mr *MultiResolution) View(data DataBlock, resolution Resolution) (*View, error)

// Generate a view tailored to an audience
func (mr *MultiResolution) ViewFor(data DataBlock, audience Audience) (*View, error)

// Get progressive disclosure levels
func (mr *MultiResolution) Levels(data DataBlock) ([]View, error)

// Register a custom view template
func (mr *MultiResolution) RegisterTemplate(resolution Resolution, template ViewTemplate) error
```

## Integration Points

- **internal/cost**: Financial views need cost data
- **internal/trust**: Trust scores in executive views
- **internal/feedback**: Operational views include production signals
- **internal/timegate**: Deadline status in all views

## TODO

- [ ] Natural language generation tuned per audience
- [ ] Chart rendering for visual formats
- [ ] Slide deck generation (PPTX/Google Slides)
- [ ] Real-time view updates (dashboard refresh)
- [ ] View preferences per user (remember what level they prefer)
- [ ] Cross-language views (generate in user's language)

## Patent Considerations

**Novel**: The audience-aware view generation system that transforms the same data block into resolution-appropriate views based on role, expertise, interests, and format preferences. The progressive disclosure system that provides 5 levels of detail from the same underlying data.
