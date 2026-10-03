package bid

import "testing"

func TestOutlineTitleKeyKeepsExistingNumberedChapter(t *testing.T) {
	for _, title := range []string{"一、项目理解", "1. 项目理解", "第一章 项目理解", "项目理解"} {
		if outlineTitleKey(title) != "项目理解" {
			t.Fatalf("wrong chapter key for %q", title)
		}
	}
	for _, title := range []string{"实施方案", "2026年项目理解", "项目理解与交付方案"} {
		if outlineTitleKey(title) == "项目理解" {
			t.Fatalf("unrelated chapter merged: %q", title)
		}
	}
}
