package metadata

import "testing"

func TestReferencePlan(t *testing.T) {
	plan := DefaultReferencePlan
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	depths := map[string]int{"11": 1, "11.16": 2, "11.16.13": 3, "11.16.13.01": 4, "11.16.13.01-01": 5,
		"11.16.13.01-01/1": 6, "11.16.13.01-01/1#1": 7, "": 0, "abc": 0, "11.16.13.01-01/x": 0, "11_16": 0}
	for code, want := range depths {
		if got := plan.Depth(code); got != want {
			t.Errorf("Depth(%q) = %d, want %d", code, got, want)
		}
	}
	children := map[string]string{"11.16.13.01": "11.16.13.01-01", "11.16.13.01-01": "11.16.13.01-01/1",
		"11.16.13.01-01/1": "11.16.13.01-01/1#1", "99.01": "99.01.01", "11": "11.01"}
	for parent, want := range children {
		if got, ok := plan.Child(parent, 1); !ok || got != want {
			t.Errorf("Child(%q, 1) = %q %v, want %q", parent, got, ok, want)
		}
	}
	if _, ok := plan.Child("11.16.13.01-01/1#1", 1); ok {
		t.Error("Child below the deepest level should fail")
	}
	if got, ok := plan.Child("11.16.13.01", 12); !ok || got != "11.16.13.01-12" {
		t.Errorf("Child 12 = %q", got)
	}
	if n, ok := plan.ChildNumber("11.16.13.01", "11.16.13.01-07"); !ok || n != 7 {
		t.Errorf("ChildNumber = %d %v", n, ok)
	}
	if _, ok := plan.ChildNumber("11.16.13.01", "11.16.13.01-07/1"); ok {
		t.Error("grandchild must not count as child")
	}
	if _, ok := plan.ChildNumber("11.16.13.01", "11.16.13.02"); ok {
		t.Error("sibling must not count as child")
	}
	parents := map[string]string{"11.16.13.01-01/1#1": "11.16.13.01-01/1", "11.16.13.01-01": "11.16.13.01", "11.16": "11", "11": "", "abc": ""}
	for code, want := range parents {
		if got := plan.ParentCode(code); got != want {
			t.Errorf("ParentCode(%q) = %q, want %q", code, got, want)
		}
	}
	bad := ReferencePlan{"{n:2}", "{n:2}"}
	if err := bad.Validate(); err == nil {
		t.Error("plan without separator should be invalid")
	}
	custom := ReferencePlan{"{n}", "-{n:3}"}
	if got, ok := custom.Child("7", 2); !ok || got != "7-002" {
		t.Errorf("custom Child = %q", got)
	}
}
