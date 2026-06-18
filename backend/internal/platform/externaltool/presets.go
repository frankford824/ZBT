package externaltool

type ProviderPreset struct {
	ProviderKey         string   `json:"provider_key"`
	Name                string   `json:"name"`
	Category            string   `json:"category"`
	Description         string   `json:"description"`
	Transport           string   `json:"transport"`
	EndpointHint        string   `json:"endpoint_hint"`
	TokenEnv            string   `json:"token_env"`
	RequiresToken       bool     `json:"requires_token"`
	ReadOnly            bool     `json:"read_only"`
	StrictAllowedTools  bool     `json:"strict_allowed_tools"`
	DefaultAllowedTools []string `json:"default_allowed_tools"`
	RecommendedUse      []string `json:"recommended_use"`
	DataBoundary        []string `json:"data_boundary"`
	SourceURL           string   `json:"source_url"`
}

func ProviderPresets() []ProviderPreset {
	presets := []ProviderPreset{
		{
			ProviderKey:        "handaas-bidding",
			Name:               "旷湖招投标大数据",
			Category:           "中国招投标数据",
			Description:        "招投标信息搜索、中标/招标/采购统计、拟建项目查询。",
			Transport:          TransportStreamableHTTP,
			EndpointHint:       "https://mcp.handaas.com/bidding/bidding_bigdata?token=...",
			TokenEnv:           externalToolTokenEnvName("handaas-bidding"),
			RequiresToken:      true,
			ReadOnly:           true,
			StrictAllowedTools: true,
			DefaultAllowedTools: []string{
				"bid_bigdata_bid_search",
				"bid_bigdata_bid_win_stats",
				"bid_bigdata_bidding_info",
				"bid_bigdata_fuzzy_search",
				"bid_bigdata_planned_projects",
				"bid_bigdata_procurement_stats",
				"bid_bigdata_tender_stats",
			},
			RecommendedUse: []string{"商机搜索", "企业中标趋势", "采购统计", "拟建项目发现"},
			DataBoundary:   []string{"只发送关键词、地区、金额、时间范围等检索条件", "禁止发送客户招标文件、投标文件、报价明细或合同正文"},
			SourceURL:      "https://github.com/handaas/bidding-mcp-server",
		},
		{
			ProviderKey:   "autorfp",
			Name:          "AutoRFP.ai",
			Category:      "RFP 响应库",
			Description:   "查询 AutoRFP.ai 项目、需求、内容库和标签，适合借鉴来源引用和 Trust Score 工作流。",
			Transport:     TransportStreamableHTTP,
			EndpointHint:  "https://api.us.autorfp.ai/mcp",
			TokenEnv:      externalToolTokenEnvName("autorfp"),
			RequiresToken: true,
			ReadOnly:      true,
			DefaultAllowedTools: []string{
				"get_project",
				"get_requirement",
				"list_projects",
				"list_requirements",
				"list_tags",
				"search_content_library",
				"usage_analytics",
			},
			RecommendedUse: []string{"历史 RFP 内容库检索", "需求来源引用模式参考", "响应复用分析"},
			DataBoundary:   []string{"只读查询外部内容库", "不得把智标通租户文件默认同步到第三方 RFP 平台"},
			SourceURL:      "https://autorfp.ai/blog/autorfp-mcp-server-launch",
		},
		{
			ProviderKey:   "qlows",
			Name:          "qlows MCP",
			Category:      "RFP 交易与公共标讯",
			Description:   "读取 RFP/bid deals、合规项、问题路由、竞争对手分析和公共 tender corpus。",
			Transport:     TransportStreamableHTTP,
			EndpointHint:  "https://app.qlows.com/api/mcp/<token>/rpc",
			TokenEnv:      externalToolTokenEnvName("qlows"),
			RequiresToken: true,
			ReadOnly:      true,
			DefaultAllowedTools: []string{
				"get_deal_snapshot",
				"get_intelligence_summary",
				"get_q_routing_state",
				"get_tender_detail",
				"list_competitors",
				"list_deals",
				"list_questions",
				"search_compliance_items",
				"search_tenders",
				"search_tenders_for_my_company",
			},
			RecommendedUse: []string{"RFP deal 快照", "合规项搜索", "公共标讯检索", "问题路由参考"},
			DataBoundary:   []string{"只读拉取外部 deal/tender 摘要", "禁止把本系统投标文件正文回写到 qlows"},
			SourceURL:      "https://github.com/getqlows/qlows-mcp",
		},
		{
			ProviderKey:        "bidcraft-compliance",
			Name:               "BidCraft Compliance Matrix",
			Category:           "RFP 合规矩阵",
			Description:        "RFP 要求抽取、章节映射、差距分析和响应提纲。",
			Transport:          TransportStreamableHTTP,
			TokenEnv:           externalToolTokenEnvName("bidcraft-compliance"),
			ReadOnly:           true,
			StrictAllowedTools: true,
			DefaultAllowedTools: []string{
				"analyze_gaps",
				"extract_requirements",
				"map_sections",
				"outline_response",
			},
			RecommendedUse: []string{"合规矩阵方法论对照", "差距分析工具边界参考"},
			DataBoundary:   []string{"只允许摘要级 RFP 文本或脱敏要求项", "生产解析和生成主路径仍由智标通内部服务完成"},
			SourceURL:      "https://github.com/crawde/mcp-bidcraft-compliance-matrix",
		},
		{
			ProviderKey:        "bidcraft-win-strategy",
			Name:               "BidCraft Win Strategy",
			Category:           "Bid/No-Bid 决策",
			Description:        "中标概率评分、price-to-win、capture planning 和 bid/no-bid 决策。",
			Transport:          TransportStreamableHTTP,
			TokenEnv:           externalToolTokenEnvName("bidcraft-win-strategy"),
			ReadOnly:           true,
			StrictAllowedTools: true,
			DefaultAllowedTools: []string{
				"bid_no_bid_decision",
				"score_win_probability",
			},
			RecommendedUse: []string{"商机评分", "投标/不投标决策", "竞争策略参考"},
			DataBoundary:   []string{"只发送机会摘要和非敏感约束", "禁止发送未脱敏报价明细、客户合同和核心资质证明"},
			SourceURL:      "https://github.com/crawde/mcp-bidcraft-win-strategy",
		},
		{
			ProviderKey:   "loopio",
			Name:          "Loopio Data API MCP",
			Category:      "企业应答库",
			Description:   "通过 Loopio Data API 读取历史答案库、项目和内容条目。",
			Transport:     TransportStreamableHTTP,
			TokenEnv:      externalToolTokenEnvName("loopio"),
			RequiresToken: true,
			ReadOnly:      true,
			DefaultAllowedTools: []string{
				"get_library_entry",
				"list_library_entries",
				"list_projects",
				"search_library",
			},
			RecommendedUse: []string{"历史答案库检索", "企业标准回复复用", "外部知识库权限继承参考"},
			DataBoundary:   []string{"只读读取已批准答案库", "不得绕过 Loopio 和智标通两侧权限控制"},
			SourceURL:      "https://github.com/fredericboyer/loopio-mcp",
		},
	}
	for index := range presets {
		presets[index].DefaultAllowedTools = normalizeAllowedTools(presets[index].DefaultAllowedTools)
	}
	return presets
}

func ProviderPresetByKey(providerKey string) (ProviderPreset, bool) {
	providerKey = normalizeProviderKey(providerKey)
	for _, preset := range ProviderPresets() {
		if preset.ProviderKey == providerKey {
			return preset, true
		}
	}
	return ProviderPreset{}, false
}
