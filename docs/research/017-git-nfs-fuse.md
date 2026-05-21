# 017 — Git-as-NFS FUSE Filesystem

> Gap: Forge Engine Infrastructure

## Problem Statement

Git repos are accessed through `git clone`, `git pull`, file I/O, then `git push`. This is 4 steps where there should be 1. What if a git repo was just a directory? `mount forge://my-repo /mnt/repo` and it's there. Read files = read from git. Write files = auto-commit. No git commands, no staging, no pushing. Just files.

## Design Decisions

### Why FUSE

FUSE (Filesystem in Userspace) lets us create virtual filesystems that the OS treats as real directories. Any application can read/write without knowing it's backed by git. The agent just uses files — the filesystem handles version control transparently.

### Transparent Versioning

Every write becomes a commit:
```
User opens /mnt/repo/src/main.go
User edits and saves
→ FUSE intercepts write
→ Auto-commits with message "auto: update src/main.go"
→ Pushes to remote (async)
→ File appears saved normally
```

### Branch as Directory

Branches are just subdirectories:
```
/mnt/repo/           → default branch (main)
/mnt/repo/.branches/feature-auth/  → feature-auth branch
/mnt/repo/.branches/fix-bug-42/    → fix-bug-42 branch
```

Creating a branch = creating a directory. Merging = copying files between directories.

### Conflict Detection

When two agents edit the same file:
1. First write wins (gets committed)
2. Second write gets a conflict file (`.conflict`)
3. Agent is notified of the conflict
4. Resolution is a new write

### Performance

- **Read cache**: Files cached in memory, invalidated on remote changes
- **Write buffer**: Batches small writes, commits every N seconds
- **Lazy clone**: Only fetches files as they're accessed (not full clone)
- **Prefetch**: Agent can hint which files it will need next

## API Surface

```go
type GitNFS struct { ... }

// Mount a git repo as a filesystem
func Mount(repoURL, mountPoint string) (*GitNFS, error)

// Unmount the filesystem
func (gn *GitNFS) Unmount() error

// Create a branch (as subdirectory)
func (gn *GitNFS) CreateBranch(name string) error

// Get filesystem stats
func (gn *GitNFS) Stats() *FSStats
```

## Integration Points

- **internal/change**: Filesystem-level conflict detection
- **internal/branch**: Git branches map to session branches
- **internal/cost**: Bandwidth and storage costs tracked
- **internal/sandbox**: Mount repos in sandbox VMs

## TODO

- [ ] Multi-remote support (GitHub + GitLab + Bitbucket)
- [ ] Lock files (prevent concurrent writes to critical files)
- [ ] Diff visualization in Forge UI
- [ ] Partial clone support (sparse checkout)
- [ ] LFS support (large file storage)
- [ ] Performance benchmarking vs traditional git

## Patent Considerations

**Novel**: Git-backed FUSE filesystem with automatic versioning, branch-as-directory semantics, and transparent push/pull. The lazy clone mechanism that fetches files on access rather than cloning the entire repository. The write buffer that batches filesystem operations into atomic git commits.
