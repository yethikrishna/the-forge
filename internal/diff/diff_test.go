package diff_test

import (
	"testing"

	"github.com/forge/sword/internal/diff"
)

func TestCompareIdentical(t *testing.T) {
	d := diff.Compare("hello\nworld\n", "hello\nworld\n", "test.txt")

	if d.Added != 0 || d.Deleted != 0 {
		t.Errorf("identical strings should have no changes: +%d -%d", d.Added, d.Deleted)
	}
}

func TestCompareAdditions(t *testing.T) {
	d := diff.Compare("hello\n", "hello\nworld\n", "test.txt")

	if d.Added != 1 {
		t.Errorf("expected 1 addition, got %d", d.Added)
	}
	if d.Deleted != 0 {
		t.Errorf("expected 0 deletions, got %d", d.Deleted)
	}
}

func TestCompareDeletions(t *testing.T) {
	d := diff.Compare("hello\nworld\n", "hello\n", "test.txt")

	if d.Deleted != 1 {
		t.Errorf("expected 1 deletion, got %d", d.Deleted)
	}
	if d.Added != 0 {
		t.Errorf("expected 0 additions, got %d", d.Added)
	}
}

func TestCompareMixed(t *testing.T) {
	old := "line1\nline2\nline3\nline4\n"
	new := "line1\nmodified\nline3\nline5\n"

	d := diff.Compare(old, new, "test.txt")

	if d.Added == 0 && d.Deleted == 0 {
		t.Error("should detect changes")
	}
}

func TestCompareEmptyOld(t *testing.T) {
	d := diff.Compare("", "hello\nworld\n", "new.txt")

	if d.Added != 2 {
		t.Errorf("expected 2 additions, got %d", d.Added)
	}
}

func TestCompareEmptyNew(t *testing.T) {
	d := diff.Compare("hello\nworld\n", "", "deleted.txt")

	if d.Deleted != 2 {
		t.Errorf("expected 2 deletions, got %d", d.Deleted)
	}
}

func TestSummary(t *testing.T) {
	d := diff.Compare("a\n", "b\n", "test.txt")
	summary := d.Summary()

	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestFormatPlain(t *testing.T) {
	d := diff.Compare("hello\n", "hello\nworld\n", "test.txt")
	formatted := diff.FormatPlain(d)

	if formatted == "" {
		t.Error("formatted diff should not be empty")
	}
	if len(formatted) < 10 {
		t.Error("formatted diff seems too short")
	}
}

func TestFormatColor(t *testing.T) {
	d := diff.Compare("hello\n", "world\n", "test.txt")
	formatted := diff.FormatColor(d)

	if formatted == "" {
		t.Error("formatted diff should not be empty")
	}
}

func TestStats(t *testing.T) {
	d := diff.Compare("a\nb\n", "c\nd\n", "test.txt")
	stats := diff.Stats(d)

	if stats == "" {
		t.Error("stats should not be empty")
	}
}

func TestPatch(t *testing.T) {
	d := diff.Compare("hello\n", "hello\nworld\n", "test.txt")
	patch := diff.Patch(d)

	if patch == "" {
		t.Error("patch should not be empty")
	}
	if len(patch) < 10 {
		t.Error("patch seems too short")
	}
}

func TestApply(t *testing.T) {
	old := "hello\n"
	new_ := "hello\nworld\n"
	d := diff.Compare(old, new_, "test.txt")

	applied := diff.Apply(old, d)
	if applied != new_ {
		t.Errorf("apply should produce new content\nexpected: %q\ngot: %q", new_, applied)
	}
}

func TestReverse(t *testing.T) {
	old := "hello\n"
	new_ := "hello\nworld\n"
	d := diff.Compare(old, new_, "test.txt")

	rd := diff.Reverse(d)
	reversed := diff.Apply(new_, rd)

	if reversed != old {
		t.Errorf("reverse apply should produce original content\nexpected: %q\ngot: %q", old, reversed)
	}
}
