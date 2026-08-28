package qualification

import "testing"

func TestNormalizeLevelOrdering(t *testing.T) {
	// 「一级及以上」这类要求靠 rank 比大小来判定，所以顺序关系本身必须成立。
	if !(RankSpecial > RankFirst && RankFirst > RankSecond && RankSecond > RankThird && RankThird > RankNone) {
		t.Fatal("等级 rank 必须严格递减：特级 > 一级 > 二级 > 三级 > 无")
	}
}

func TestNormalizeLevel(t *testing.T) {
	cases := []struct {
		in        string
		wantLevel string
		wantRank  int
	}{
		{"市政公用工程施工总承包一级", "一级", RankFirst},
		{"建筑工程施工总承包特级", "特级", RankSpecial},
		{"公路养护工程施工资质甲级", "一级", RankFirst},
		{"建筑业企业二级资质", "二级", RankSecond},
		{"地基基础工程专业承包三级", "三级", RankThird},
		{"市政公用工程乙级", "二级", RankSecond},
		{"检测机构丙级", "三级", RankThird},
		{"安全生产许可证", "", RankNone},
		{"施工劳务资质", "", RankNone},
		{"", "", RankNone},
		// 升级表述里同时出现两个等级，取高的那个才与证书实际效力一致
		{"由二级升为一级资质", "一级", RankFirst},
	}
	for _, c := range cases {
		gotLevel, gotRank := NormalizeLevel(c.in)
		if gotLevel != c.wantLevel || gotRank != c.wantRank {
			t.Errorf("NormalizeLevel(%q) = (%q, %d), want (%q, %d)",
				c.in, gotLevel, gotRank, c.wantLevel, c.wantRank)
		}
	}
}

func TestMapCategorySkipsIrrelevant(t *testing.T) {
	// 这三类占 zizhi 库一半以上，但与投标资质匹配无关，必须不同步，
	// 否则待确认队列会被两千多条身份证和社保回单淹没。
	for _, c := range []string{"legal_id", "tax_social", "other", "charter", "intro"} {
		if _, ok := MapCategory(c); ok {
			t.Errorf("分类 %q 不应同步到资质档案", c)
		}
	}
}

func TestMapCategoryTargets(t *testing.T) {
	cases := map[string]string{
		"safety_permit":               "certificate",
		"construction_grade_2":        "certificate",
		"highway_maintenance_grade_a": "certificate",
		"personnel":                   "personnel",
		"performance":                 "performance",
		"audit":                       "financial",
	}
	for cat, want := range cases {
		m, ok := MapCategory(cat)
		if !ok {
			t.Fatalf("分类 %q 应当可同步", cat)
		}
		if m.Target != want {
			t.Errorf("分类 %q 目标为 %q, want %q", cat, m.Target, want)
		}
	}
	// 分类名本身隐含等级的，必须带出正确 rank
	if m, _ := MapCategory("construction_grade_2"); m.DefaultRank != RankSecond {
		t.Errorf("建筑业二级资质应映射为 RankSecond, got %d", m.DefaultRank)
	}
	if m, _ := MapCategory("highway_maintenance_grade_a"); m.DefaultRank != RankFirst {
		t.Errorf("公路养护甲级应映射为 RankFirst（甲级等同一级）, got %d", m.DefaultRank)
	}
}

func TestLooksLikeNonCertificate(t *testing.T) {
	// 实测 zizhi 把「项申请表.jpg」判成了有效期 2019 的安全管理人员证书，
	// 这类文件必须能被识别出来并压低置信度。
	suspicious := []string{
		"项申请表.jpg", "资质申报材料.pdf", "安许申请表.docx",
		"证书模板.doc", "空白表格.xlsx", "人员汇总表.xls",
		"目录.pdf", "封面.jpg", "承诺书.pdf", "12345.jpg", "",
	}
	for _, n := range suspicious {
		if !LooksLikeNonCertificate(n) {
			t.Errorf("%q 应被判为非证书原件", n)
		}
	}
	legit := []string{
		"安许2029.2.24.pdf", "劳务资质.pdf", "一级建造师证书 2027.2.pdf",
		"营业执照.jpg", "ISO9001认证证书.pdf",
	}
	for _, n := range legit {
		if LooksLikeNonCertificate(n) {
			t.Errorf("%q 不应被判为非证书", n)
		}
	}
}

func TestLooksLikeNonPersonName(t *testing.T) {
	// 实测 zizhi 聚合出的「人员」里混进了这些，必须能识别出来
	noise := []string{
		"中娣手持", "人倪修江", "全日制", "倪总",
		"身份证正反面", "张三复印件", "保通建设有限公司", "",
		"李", "王小明的身份证扫描件", "abc", "张三2",
	}
	for _, n := range noise {
		if !LooksLikeNonPersonName(n) {
			t.Errorf("%q 应被判为非人名", n)
		}
	}
	// 真实姓名不能被误杀，误杀会让真的建造师从档案里消失
	real := []string{"倪亮", "冯卫军", "刘惠", "吴笑融", "欧阳修文"}
	for _, n := range real {
		if LooksLikeNonPersonName(n) {
			t.Errorf("%q 是正常姓名，不应被过滤", n)
		}
	}
}

func TestConfidenceRanking(t *testing.T) {
	// 字段越全置信度越高；文件名可疑要显著降权。
	full := Confidence("(苏)JZ安许证字[2011]090012", "2029-02-24", "江苏省住房和城乡建设厅", "保通建设工程有限公司", false)
	partial := Confidence("", "2027-05-14", "", "", false)
	bare := Confidence("", "", "", "", false)
	suspicious := Confidence("", "2019-01-31", "", "", true)

	if !(full > partial && partial > bare) {
		t.Errorf("置信度应随字段完整度递增: full=%.2f partial=%.2f bare=%.2f", full, partial, bare)
	}
	if suspicious >= partial {
		t.Errorf("可疑文件名应降低置信度: suspicious=%.2f partial=%.2f", suspicious, partial)
	}
	if full > 0.99 || bare < 0.05 {
		t.Errorf("置信度必须落在 [0.05, 0.99]: full=%.2f bare=%.2f", full, bare)
	}
}
