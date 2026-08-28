package qualification

import "testing"

// applyLevel 必须保留人工填写的等级原文。各资质体系的术语不通用：
// 公路养护资质分甲乙丙级、建筑业企业资质分一二三级，categoryMappings 里
// 就是按各自的行业说法配的。若哪天有人把这里改回 NormalizeLevel 的归一文本，
// 审核过的公路养护资质会显示成「一级」，与同类记录不一致。
func TestApplyLevelKeepsOriginalTerm(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantText string
		wantRank int
	}{
		{"公路养护资质用甲级", "甲级", "甲级", RankFirst},
		{"建筑业资质用一级", "一级", "一级", RankFirst},
		{"乙级与二级同级", "乙级", "乙级", RankSecond},
		{"特级最高", "特级", "特级", RankSpecial},
		{"识别不出的写法照样存，但不参与比较", "A 类", "A 类", RankNone},
		{"留空表示原件上没有等级", "", "", RankNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newUpdateBuilder("some-id")
			applyLevel(b, tc.input)
			// args[0] 是 where 用的 id，之后依次是 cert_level、cert_level_rank
			if got := b.args[1]; got != tc.wantText {
				t.Fatalf("cert_level = %q, want %q", got, tc.wantText)
			}
			if got := b.args[2]; got != tc.wantRank {
				t.Fatalf("cert_level_rank = %v, want %d", got, tc.wantRank)
			}
		})
	}
}

// 同一类资质在台账里只应有一种等级写法。曾出现过 12 条公路养护资质存「甲级」、
// 1 条存「一级」的情况：后者的文件名里带了等级字样，NormalizeLevel 把它归一成了
// 「一级」，盖掉分类的行业术语，rank 一样却像两类资质。
func TestResolveLevelPrefersCategoryTerm(t *testing.T) {
	highway, ok := MapCategory("highway_maintenance_grade_a")
	if !ok {
		t.Fatal("highway_maintenance_grade_a should be mapped")
	}
	grade2, _ := MapCategory("construction_grade_2")
	unleveled, _ := MapCategory("safety_permit")

	cases := []struct {
		name     string
		text     string
		mapping  CategoryMapping
		wantText string
		wantRank int
	}{
		{"文本无等级时用分类术语", "公路养护资质 养护资质证书.pdf", highway, "甲级", RankFirst},
		{"文本写甲级，同级，仍用分类术语", "公路养护甲级资质.pdf", highway, "甲级", RankFirst},
		{"文本写一级，与甲级同级，仍用分类术语", "公路养护一级资质.pdf", highway, "甲级", RankFirst},
		{"文本明确写二级，与分类不同级，以证书为准", "公路养护二级资质.pdf", highway, "二级", RankSecond},
		{"建筑业分类用一二三级", "建筑业企业资质.pdf", grade2, "二级", RankSecond},
		{"不分级的分类照常从文本判定", "安全生产许可证.pdf", unleveled, "", RankNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, rank := ResolveLevel(tc.text, tc.mapping)
			if text != tc.wantText || rank != tc.wantRank {
				t.Fatalf("ResolveLevel(%q) = (%q, %d), want (%q, %d)",
					tc.text, text, rank, tc.wantText, tc.wantRank)
			}
		})
	}
}

func TestNormalizeVerifyStatus(t *testing.T) {
	for _, valid := range []string{"pending_review", "confirmed", "rejected"} {
		if _, err := normalizeVerifyStatus(valid); err != nil {
			t.Fatalf("normalizeVerifyStatus(%q) returned %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "approved", "CONFIRMED", "confirmed; drop table"} {
		if _, err := normalizeVerifyStatus(invalid); err == nil {
			t.Fatalf("normalizeVerifyStatus(%q) accepted an invalid status", invalid)
		}
	}
}

// 空串与 nil 在这里含义不同：nil 表示不改这一列（调用方根本不会调到这里），
// 空串表示把它清空——审核时删掉抽错的有效期是常规操作。
func TestReviewDate(t *testing.T) {
	empty := ""
	got, err := reviewDate(&empty)
	if err != nil {
		t.Fatalf("empty date returned %v", err)
	}
	if got != nil {
		t.Fatalf("empty date = %v, want nil so the column is cleared", got)
	}

	valid := "2029-02-24"
	got, err = reviewDate(&valid)
	if err != nil || got == nil {
		t.Fatalf("valid date returned (%v, %v)", got, err)
	}

	for _, bad := range []string{"2029/02/24", "24-02-2029", "2029-2-4x", "tomorrow"} {
		if _, err := reviewDate(&bad); err == nil {
			t.Fatalf("reviewDate(%q) accepted a malformed date", bad)
		}
	}
}

func TestReviewTextRejectsOverlongInput(t *testing.T) {
	long := make([]rune, maxReviewTextRunes+1)
	for i := range long {
		long[i] = '证'
	}
	value := string(long)
	if _, err := reviewText(&value); err == nil {
		t.Fatal("reviewText accepted input longer than the column is meant to hold")
	}

	padded := "  江苏省住建厅  "
	got, err := reviewText(&padded)
	if err != nil {
		t.Fatalf("reviewText returned %v", err)
	}
	if got != "江苏省住建厅" {
		t.Fatalf("reviewText = %q, want the trimmed value", got)
	}
}
