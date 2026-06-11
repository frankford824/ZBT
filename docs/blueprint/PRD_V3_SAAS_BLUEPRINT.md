# 智标通 V3 SaaS 蓝图

## 产品定位

智标通是面向企业投标团队的 B2B 智能标书生成 SaaS 平台。平台目标不是单功能 AI 写标书工具，而是覆盖标讯、项目、标书、知识库、合规、审批、成本和团队协作的完整业务链路。

## 业务链路

标讯大厅 -> 招标文件解析 -> 创建项目 -> 新建标书 -> AI 分步生成标书 -> 知识库 RAG 引用 -> 逐章生成与人工确认 -> 标书编辑器 -> 合规检查 -> 人工审核 / 审批 -> Word / PDF / ZIP 导出 -> 提交投标 -> 开标结果 -> 中标后创建成本项目 -> 成本分析 -> 中标案例回流知识库。

## 必保留模块

1. 工作台
2. 标讯大厅
3. 标书生成
4. 合规检查
5. 项目管理
6. 成本管理
7. 知识库
8. 团队协作

## 页面范围

以 V2.4 为页面基准，落地 8 个主页面、6 个子页面和必要详情路由。HTML 原型只作为视觉和布局参考，不作为路由和数据模型事实源。

## 架构裁决

前端使用 React + TypeScript + Vite + React Router + TanStack Query + Zustand + Ant Design + Tiptap + ECharts + React Hook Form + Zod。

主业务后端使用 Golang + Gin + pgx + sqlc + goose + Asynq，PostgreSQL 16 开启 RLS 和 pgvector。禁止 GORM、Drools、Rust 主后端。

AI / RAG / 文档处理服务使用 Python + FastAPI + Pydantic v2，模型调用统一经过 ModelRouter。文档解析、OCR、RAG、章节生成、合规语义检查和导出在 Python 服务执行，完成后回调 Go，由 Go 验签落库并推进状态机。

## 一期重点

1. SaaS 底座先行：tenant、user、member、role、permission、audit、notification、file、ai_call_logs。
2. 所有业务表带 tenant_id，并启用 RLS。
3. 所有 API 后端强校验权限，前端菜单隐藏只是体验优化。
4. 分离标书一期支持技术标 / 商务标生成、编辑、分别导出和 ZIP 打包。
5. Word docx 最小导出链路必须提前验证。
6. MockProvider 必须可跑通无真实 API Key 的平台链路。
