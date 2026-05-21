# Forge in 60 Seconds — Demo Script

> **Recording target**: <60 seconds from prompt to "first agent running with governance"  
> **Format**: Clean terminal recording (asciinema or screen capture)  
> **Prerequisites**: Forge installed, Ollama available (or just show the commands)

---

## Setup (before recording)

```bash
# Clean terminal, large font, dark theme
# Forge binary on PATH: export PATH=$PATH:~/go/bin
# Optional: clear shell history for clean tab completions
clear
```

---

## The 5-Command Demo

### Command 1 — Health check & auto-fix (~5s)
```bash
forge doctor --fix
```
**Expected output**: All checks pass. Go SDK found, WAL state clean, environment ready.

---

### Command 2 — Zero-cloud project init (~5s)
```bash
forge init --local
```
**Expected output**: Forgefile created with Ollama/DeepSeek R1 preset. `.forge/` directory ready.  
**Talking point**: "No API keys. No cloud. Your data never leaves your machine."

---

### Command 3 — Interactive tutorial (~5s)
```bash
forge learn 0
```
**Expected output**: Lesson starts, step 1 shown.  
**Talking point**: "Built-in interactive tutorials. `forge learn list` shows all 7."

---

### Command 4 — Governance assessment (~10s)
```bash
forge govern assess --name demo
```
**Expected output**: Governance score across security/compliance/cost/audit/resilience/ethics.  
**Talking point**: "Every agent system gets a governance score. Not a checkbox — a measurable practice."

Optionally show:
```bash
forge catalog list          # registered agents
forge cost budget --set 10  # spend cap
```

---

### Command 5 — Live cost dashboard (~5s)
```bash
forge cost live
```
**Expected output**: Real-time dashboard. Local models show `$0.00`.  
**Talking point**: "Full cost transparency. Local models: free. Cloud models: budgeted and audited."

---

## Caption Overlay Suggestions

| Time | Caption |
|------|---------|
| 0:00 | "Self-hosted AI orchestration. Governance-first." |
| 0:10 | "Zero cloud. Ollama + DeepSeek R1." |
| 0:25 | "Every agent gets a governance score." |
| 0:45 | "Local models: $0.00. No surprises." |
| 0:58 | "forge — self-hosted, governed, MCP-native" |

---

## Full Script (with `forge quickstart --demo`)

Run this to see the exact demo flow without recording:

```bash
forge quickstart --demo
```

Or list the steps:

```bash
forge quickstart --demo --list
```

---

## Post-Recording Checklist

- [ ] Edit to <60s (cut dead time between commands)
- [ ] Add captions from table above
- [ ] Upload to YouTube: title "Forge in 60 Seconds — Self-Hosted AI Orchestration"
- [ ] Post X/Twitter thread with video + link to README
- [ ] Update README hero with video embed + one-liner install
- [ ] Hashtags: `#TheForge #MCP #SelfHostedAI #AgentOrchestration #LocalAI #OpenSource`

---

## One-Liner Install (for video description)

```bash
# Install (when published)
brew install the-forge/tap/forge
# or
curl -sSL https://get.forge.dev | sh

# Then:
forge quickstart --demo
```

---

*Demo validated by Forge Coder — all 5 commands exist and work as documented.*  
*`forge quickstart --demo` runs the identical flow interactively.*
