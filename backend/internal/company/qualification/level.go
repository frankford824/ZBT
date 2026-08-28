// Package qualification 维护企业资质档案，并从局域网 zizhi-api（NAS 资质库检索服务）同步。
//
// 本文件只做确定性映射，不调用任何模型。原因见 docs/ZBT-INTEGRATION-PLAN.md 6.8：
// 招标要求「一级及以上」而企业持有「特级」，字符串比不出大小，必须先归一化成可比整数。
// 这一步一旦出错，会把能投的标判成不能投（丢单）或反之（废标），代价比抽不出来高得多，
// 所以映射表写死在服务端并由单元测试守住，不交给 LLM 判断。
package qualification

import (
	"regexp"
	"strings"
)

// 资质等级归一化后的可比整数。数值本身无业务含义，只用于比较大小。
const (
	RankNone    = 0 // 不分级（劳务资质、安全生产许可证、体系认证等）
	RankThird   = 1 // 三级 / 丙级
	RankSecond  = 2 // 二级 / 乙级
	RankFirst   = 3 // 一级 / 甲级
	RankSpecial = 4 // 特级
)

// levelPatterns 按 rank 从高到低匹配：文本里同时出现「一级」和「二级」时
// （例如「由二级升为一级」），取更高的那个，与证书实际效力一致。
var levelPatterns = []struct {
	rank    int
	display string
	words   []string
}{
	{RankSpecial, "特级", []string{"特级"}},
	{RankFirst, "一级", []string{"一级", "甲级", "壹级"}},
	{RankSecond, "二级", []string{"二级", "乙级", "贰级"}},
	{RankThird, "三级", []string{"三级", "丙级", "叁级"}},
}

// NormalizeLevel 从证书名称或正文片段中提取资质等级，返回展示用文本与可比 rank。
// 取不到等级时返回空串与 RankNone —— 不分级的资质（如安全生产许可证）本就没有等级，
// 这与「抽取失败」在下游是同一种处理：不参与等级比较。
func NormalizeLevel(text string) (string, int) {
	if text == "" {
		return "", RankNone
	}
	for _, p := range levelPatterns {
		for _, w := range p.words {
			if strings.Contains(text, w) {
				return p.display, p.rank
			}
		}
	}
	return "", RankNone
}

// CategoryMapping 描述 zizhi 的一个分类如何落到 ZBT 的资质档案。
type CategoryMapping struct {
	// Target 决定进哪张表：certificate / personnel / performance / financial。
	// 空串表示该分类与投标资质匹配无关，不同步。
	Target string
	// CertCategory 是 ZBT 侧的资质大类，仅 Target=certificate 时有意义。
	CertCategory string
	// DefaultLevel 用于分类名本身已隐含等级的情况（如「建筑业二级资质」）。
	DefaultLevel string
	DefaultRank  int
}

// categoryMappings 把 zizhi 的分类映射到 ZBT 资质档案。
//
// 未列出的分类一律不同步。特别是 legal_id(378)、tax_social(963)、other(1612) 这三类
// 占了 zizhi 库的一半以上，但它们是法人身份证件、税务社保回单和杂项，
// 与投标资质匹配无关，全量导入只会把待确认队列淹掉。
var categoryMappings = map[string]CategoryMapping{
	"license":                     {Target: "certificate", CertCategory: "营业执照"},
	"safety_permit":               {Target: "certificate", CertCategory: "安全生产许可证"},
	"labor_qualify":               {Target: "certificate", CertCategory: "施工劳务资质"},
	"iso":                         {Target: "certificate", CertCategory: "体系认证"},
	"credit":                      {Target: "certificate", CertCategory: "信用资信"},
	"honor":                       {Target: "certificate", CertCategory: "荣誉证书"},
	"bank_permit":                 {Target: "certificate", CertCategory: "开户许可"},
	"taxpayer":                    {Target: "certificate", CertCategory: "一般纳税人"},
	"equipment":                   {Target: "certificate", CertCategory: "设备"},
	"construction_grade_2":        {Target: "certificate", CertCategory: "建筑业企业资质", DefaultLevel: "二级", DefaultRank: RankSecond},
	"construction_grade_3":        {Target: "certificate", CertCategory: "建筑业企业资质", DefaultLevel: "三级", DefaultRank: RankThird},
	"highway_maintenance_grade_a": {Target: "certificate", CertCategory: "公路养护资质", DefaultLevel: "甲级", DefaultRank: RankFirst},
	"personnel":                   {Target: "personnel"},
	"performance":                 {Target: "performance"},
	"audit":                       {Target: "financial"},
}

// MapCategory 返回某个 zizhi 分类的同步目标；ok=false 表示该分类不同步。
func MapCategory(category string) (CategoryMapping, bool) {
	m, ok := categoryMappings[category]
	return m, ok
}

// ResolveLevel 决定一条证书入档时的展示等级与可比 rank。
//
// 等级优先从证书自身文本判定，判不出来才退到分类隐含的等级。但两者等级相同时
// 以分类的说法为准：各资质体系的术语不通用，公路养护资质分甲乙丙级，而
// NormalizeLevel 会把「甲级」归一成「一级」。若不做这一步，同一类资质里
// 文件名带等级字样的存「一级」、不带的存「甲级」，rank 明明一样，看着却像两回事。
//
// 文本判出的等级与分类默认不同时（分类是甲级、文件名却明确写着二级），
// 以证书自身为准——分类只是个粗分桶，证书上印的才算。
func ResolveLevel(text string, mapping CategoryMapping) (string, int) {
	level, rank := NormalizeLevel(text)
	if mapping.DefaultRank != RankNone && (rank == RankNone || rank == mapping.DefaultRank) {
		return mapping.DefaultLevel, mapping.DefaultRank
	}
	return level, rank
}

// nonCertificateHints 命中即判定「这不是一张证书本身」。
//
// 这条过滤是实测倒逼出来的：zizhi 的 /expiring 首条返回「项申请表.jpg」，
// 被判成「专职安全生产管理人员」证书且有效期 2019-01-31。申请表、申报材料、
// 空白模板都会带上证书的关键词和日期，抽取器区分不了，但文件名能区分。
var nonCertificateHints = []string{
	"申请表", "申报表", "申报材料", "申请材料", "登记表", "审批表",
	"模板", "范本", "样表", "空白", "示例",
	"目录", "封面", "汇总表", "清单", "台账",
	"承诺书", "说明", "情况表",
}

var pureDigitsRe = regexp.MustCompile(`^[\d\s\-_.]+$`)

// LooksLikeNonCertificate 判断文件名是否明显不是证书原件。
// 命中的记录仍会同步（用户可能确实需要看到），但会被压低置信度并强制人工确认。
func LooksLikeNonCertificate(name string) bool {
	if name == "" {
		return true
	}
	base := name
	if i := strings.LastIndex(base, "."); i > 0 {
		base = base[:i]
	}
	if pureDigitsRe.MatchString(base) {
		return true
	}
	for _, h := range nonCertificateHints {
		if strings.Contains(base, h) {
			return true
		}
	}
	return false
}

// nonPersonNameHints 是人名字段里出现即说明「这不是一个人名」的词。
//
// zizhi 的人员是从 NAS 目录名和证件 OCR 结果里聚合出来的，实测混进了
// 「中娣手持」「人倪修江」「全日制」「倪总」这类条目：有的是扫描件描述词被
// 当成了名字，有的是 OCR 把相邻文字粘了上去，有的是称谓或学历。
// 这些条目仍会入库（背后可能真有个人），但要降到最低置信度排到审核队首。
var nonPersonNameHints = []string{
	"手持", "正反", "反面", "正面", "复印", "扫描", "照片", "原件", "彩色",
	"全日制", "身份证", "劳动合同", "证书", "附件", "文件",
	"公司", "有限", "集团", "部门", "项目部",
	"总", "经理", "主任",
}

// LooksLikeNonPersonName 判断姓名字段是否不像一个自然人姓名。
// 中文姓名通常 2~4 字；过长、过短或含有上述噪声词的，基本可以断定是抽取残留。
func LooksLikeNonPersonName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	for _, h := range nonPersonNameHints {
		if strings.Contains(name, h) {
			return true
		}
	}
	r := []rune(name)
	// 纯中文姓名超过 4 字多半是把两段文字粘在了一起；1 字则是截断残留。
	if len(r) < 2 || len(r) > 4 {
		return true
	}
	for _, ch := range r {
		if ch < 0x4e00 || ch > 0x9fff {
			// 掺入数字、字母或标点的，同样不是干净的姓名
			return true
		}
	}
	// 四字姓名要么是复姓（「欧阳修文」），要么是 OCR 把前一个字粘了上来
	// ——实测的「人倪修江」就是从「经办人倪修江」截出来的。父母双姓组合
	// （「李王思远」）会被一并压到队首，这是可接受的：这类名字本就值得人眼
	// 过一遍，且判错的代价只是排序靠前，记录照样入库。
	if len(r) == 4 && !startsWithCompoundSurname(name) {
		return true
	}
	return false
}

// compoundSurnames 只收现代仍在使用的复姓，生僻的历史复姓不列——
// 列进来反而会放过更多 OCR 粘连。
var compoundSurnames = []string{
	"欧阳", "司马", "上官", "夏侯", "诸葛", "东方", "皇甫", "尉迟",
	"公孙", "轩辕", "令狐", "宇文", "长孙", "慕容", "司徒", "司空",
	"端木", "南宫", "万俟", "闻人", "赫连", "拓跋", "百里", "呼延",
	"独孤", "澹台", "淳于", "单于", "太叔", "申屠", "钟离", "鲜于",
	"仲孙", "第五", "东郭", "西门", "南门", "谷梁", "乐正", "宗政",
	"濮阳", "闾丘", "梁丘", "左丘",
}

func startsWithCompoundSurname(name string) bool {
	for _, s := range compoundSurnames {
		if strings.HasPrefix(name, s) {
			return true
		}
	}
	return false
}

// Confidence 根据 zizhi 抽出的字段完整度给出置信度。
//
// 分档依据来自实测：4809 个文件里只有 999 条有 holder、410 条有 valid_to，
// 也就是说绝大多数文件只有分类和全文。有证号又有有效期又有发证机关的，
// 基本可以确认是证书原件正面；只有分类的，很可能是扫描件的某一页或附件。
// 置信度只影响待确认队列的排序，不改变「必须人工确认」这个前提。
func Confidence(certNo, validTo, issuer, holder string, suspiciousName bool) float64 {
	score := 0.2
	if certNo != "" {
		score += 0.3
	}
	if validTo != "" {
		score += 0.25
	}
	if issuer != "" {
		score += 0.15
	}
	if holder != "" {
		score += 0.1
	}
	if suspiciousName {
		score -= 0.35
	}
	if score < 0.05 {
		score = 0.05
	}
	if score > 0.99 {
		score = 0.99
	}
	return score
}
