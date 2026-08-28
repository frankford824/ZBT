package qualification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ZizhiClient 访问局域网内的 zizhi-api（NAS 资质库全文检索服务）。
//
// 该服务把公司 NAS 上的资质扫描件做了 OCR、分类和字段抽取，并按小时增量维护。
// ZBT 只读它，不写它：资质原件的权威副本留在 NAS，ZBT 侧存的是结构化档案与来源引用。
type ZizhiClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// searchLimit 是 zizhi 允许的单次返回上限，超过会直接 422。
//
// 该接口只有 limit 没有 offset，所以无法翻页，只能靠「一次取完」。
// 好在开启 collapse_pages 合并多页扫描件、排除失效索引之后，
// 各证书分类的有效条目都远低于 100（例如 credit 的 116 个文件合并后只剩 37 条），
// 因此按分类逐类拉取是够用的。人员接口会超过 100，用 has_expired 分片处理。
const searchLimit = 100

var ErrZizhiNotConfigured = errors.New("zizhi api is not configured")

func NewZizhiClient(baseURL, apiKey string) *ZizhiClient {
	return &ZizhiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		// OCR 后的全文可能较大，且服务与 ZBT 同机房内网，超时给宽一些。
		http: &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *ZizhiClient) Configured() bool {
	return c != nil && c.baseURL != "" && c.apiKey != ""
}

// ZizhiFile 是 zizhi 检索结果里的一条文件记录。
// 字段名与 zizhi 返回保持一致，避免在映射层之外再做一次改名。
type ZizhiFile struct {
	ID            int64   `json:"id"`
	Path          string  `json:"path"`
	Name          string  `json:"name"`
	Ext           string  `json:"ext"`
	Size          int64   `json:"size"`
	Category      string  `json:"category"`
	CategoryLabel string  `json:"category_label"`
	Stale         bool    `json:"stale"`
	ContentState  string  `json:"content_state"`
	Holder        string  `json:"holder"`
	CertNo        string  `json:"cert_no"`
	CertKind      string  `json:"cert_kind"`
	ValidFrom     string  `json:"valid_from"`
	ValidTo       string  `json:"valid_to"`
	Issuer        string  `json:"issuer"`
	Company       string  `json:"company"`
	Expired       bool    `json:"expired"`
	PersonID      int64   `json:"person_id"`
	UNCPath       string  `json:"unc_path"`
	Snippet       string  `json:"snippet"`
	Score         float64 `json:"score"`
}

// ZizhiPerson 是 zizhi 聚合出的人员档案。
type ZizhiPerson struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Company       string  `json:"company"`
	FileCount     int     `json:"file_count"`
	CertKinds     string  `json:"cert_kinds"`
	ExpiredCount  int     `json:"expired_count"`
	LatestValidTo string  `json:"latest_valid_to"`
	UpdatedAt     float64 `json:"updated_at"`
}

// ZizhiHealth 是 /health 的摘要，用于在 ZBT 前端展示资质库的连通性与规模。
type ZizhiHealth struct {
	OK           bool           `json:"ok"`
	Count        int            `json:"count"`
	ByCategory   map[string]int `json:"by_category"`
	Expired      int            `json:"expired"`
	People       int            `json:"people"`
	Indexing     bool           `json:"indexing"`
	Extracting   bool           `json:"extracting"`
	OCRAvailable bool           `json:"ocr_available"`
	LastIndexAt  string         `json:"last_index_at"`
}

func (c *ZizhiClient) get(ctx context.Context, path string, q url.Values, out any) error {
	if !c.Configured() {
		return ErrZizhiNotConfigured
	}
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("zizhi request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 不回显响应体：其中可能含有 NAS 路径等内网信息。
		return fmt.Errorf("zizhi returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *ZizhiClient) Health(ctx context.Context) (*ZizhiHealth, error) {
	var h ZizhiHealth
	if err := c.get(ctx, "/health", nil, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// SearchByCategory 拉取某个分类下的全部文件。
//
// collapse_pages=true 让多页扫描件合并成一条，否则 1094 个分页图片会变成
// 上千条重复的「资质证书」记录。include_stale=false 排除索引已失效的条目。
func (c *ZizhiClient) SearchByCategory(ctx context.Context, category string) ([]ZizhiFile, error) {
	q := url.Values{}
	q.Set("category", category)
	q.Set("limit", strconv.Itoa(searchLimit))
	q.Set("collapse_pages", "true")
	q.Set("include_stale", "false")
	q.Set("snippet", "true")

	var resp struct {
		Count int         `json:"count"`
		Items []ZizhiFile `json:"items"`
	}
	if err := c.get(ctx, "/v1/zizhi/search", q, &resp); err != nil {
		return nil, err
	}
	if len(resp.Items) >= searchLimit {
		// 顶到上限说明可能还有没取到的，但接口没有 offset 可翻页。
		// 报错而不是静默返回部分结果：资质档案不全会直接导致匹配判错。
		return resp.Items, fmt.Errorf("分类 %s 命中数达到接口上限 %d，结果可能不完整", category, searchLimit)
	}
	return resp.Items, nil
}

// People 拉取全部人员档案。
//
// 人员总数（155）超过单次 100 条的上限且接口无法翻页，所以用 has_expired
// 把结果切成「有过期证件的」和「没有的」两批分别取，再按 id 合并去重。
// 这两批互斥且完备，合起来正好覆盖全部人员。
func (c *ZizhiClient) People(ctx context.Context) ([]ZizhiPerson, error) {
	seen := map[int64]bool{}
	var all []ZizhiPerson

	for _, hasExpired := range []string{"true", "false"} {
		q := url.Values{}
		q.Set("limit", strconv.Itoa(searchLimit))
		q.Set("has_expired", hasExpired)

		var resp struct {
			Count int           `json:"count"`
			Items []ZizhiPerson `json:"items"`
		}
		if err := c.get(ctx, "/v1/zizhi/people", q, &resp); err != nil {
			return all, err
		}
		if len(resp.Items) >= searchLimit {
			return all, fmt.Errorf("人员分片 has_expired=%s 达到接口上限 %d，结果可能不完整", hasExpired, searchLimit)
		}
		for _, p := range resp.Items {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			all = append(all, p)
		}
	}
	return all, nil
}

// SourceRefFile / SourceRefPerson 生成同步幂等键，与迁移 00037 的唯一索引配合使用。
func SourceRefFile(id int64) string   { return "zizhi:file:" + strconv.FormatInt(id, 10) }
func SourceRefPerson(id int64) string { return "zizhi:person:" + strconv.FormatInt(id, 10) }

// parseDate 把 zizhi 的日期字符串转成可入库的值。
// zizhi 对抽不到的日期返回空串而非 null，且偶有 "2029.02.24" 这类分隔符变体。
func parseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02", "2006/01/02", "2006.01.02", "20060102"} {
		if t, err := time.Parse(layout, s); err == nil {
			// 明显越界的日期多半是抽错（把证号或页码认成了年份），宁可丢弃也不入库，
			// 否则会污染「按投标截止日判定资质是否有效」的计算。
			if t.Year() < 1980 || t.Year() > 2100 {
				return nil
			}
			return &t
		}
	}
	return nil
}
