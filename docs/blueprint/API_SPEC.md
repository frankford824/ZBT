# API 规格

统一前缀：/api/v1。

## 基础

GET /healthz，GET /api/v1/meta/routes。

## Auth

POST /auth/register、POST /auth/login、POST /auth/refresh、POST /auth/logout、GET /me。

## Tenant / Team

GET /tenant、PATCH /tenant、GET /tenant/members、POST /tenant/members/invite、PATCH /tenant/members/:id、DELETE /tenant/members/:id、GET /roles、POST /roles、PATCH /roles/:id、DELETE /roles/:id。

## Tender

GET /tenders、POST /tenders、GET /tenders/:id、PATCH /tenders/:id、POST /tenders/:id/favorite、DELETE /tenders/:id/favorite、POST /tenders/:id/create-project、POST /tenders/:id/create-bid、GET /tender-sources、POST /tender-sources、PATCH /tender-sources/:id、DELETE /tender-sources/:id、POST /tender-sources/:id/verify。

## Project

GET /projects、POST /projects、GET /projects/:id、PATCH /projects/:id、DELETE /projects/:id、POST /projects/:id/transition、GET /projects/:id/milestones、POST /projects/:id/milestones、PATCH /projects/:id/milestones/:milestoneId、DELETE /projects/:id/milestones/:milestoneId、POST /projects/:id/members、DELETE /projects/:id/members/:memberId、POST /projects/:id/create-cost-project、GET /projects/:id/activities。

## Bid

GET /bids、POST /bids、GET /bids/:id、PATCH /bids/:id、DELETE /bids/:id、POST /bids/:id/upload-tender-file、POST /bids/:id/parse-tender、GET /bids/:id/parse-result、PUT /bids/:id/parse-result、POST /bids/:id/outline/generate、GET /bids/:id/parts、GET /bids/:id/parts/:partId/outline、PUT /bids/:id/parts/:partId/outline、GET /bids/:id/material-selection、PUT /bids/:id/material-selection、POST /bids/:id/generate、GET /bids/:id/generation-jobs、GET /generation-jobs/:jobId、POST /generation-jobs/:jobId/pause、POST /generation-jobs/:jobId/resume、POST /generation-jobs/:jobId/cancel、GET /bids/:id/generation/stream、GET /bids/:id/chapters、PATCH /chapters/:chapterId、POST /chapters/:chapterId/accept、POST /chapters/:chapterId/regenerate、GET /chapters/:chapterId/versions、GET /chapters/:chapterId/diff、PUT /chapters/:chapterId/content、POST /chapters/:chapterId/ai-action、POST /bids/:id/exports、GET /bid-exports/:exportId、GET /bid-templates、POST /bid-templates/:templateId/use。

## Knowledge

GET /knowledge、GET /knowledge/categories、POST /knowledge/categories、PATCH /knowledge/categories/:id、DELETE /knowledge/categories/:id、GET /knowledge/tags、POST /knowledge/tags、PATCH /knowledge/tags/:id、DELETE /knowledge/tags/:id、GET /knowledge/documents、POST /knowledge/documents、GET /knowledge/documents/:id、PATCH /knowledge/documents/:id、DELETE /knowledge/documents/:id、POST /knowledge/documents/:id/process、GET /knowledge/documents/:id/preview、GET /knowledge/documents/:id/references、POST /knowledge/search、GET /knowledge/templates、POST /knowledge/templates、GET /knowledge/stats。

## Compliance

POST /compliance/checks、GET /compliance/checks、GET /compliance/checks/:id、GET /compliance/checks/:id/issues、GET /compliance/checks/:id/stream、POST /compliance/issues/:id/autofix、POST /compliance/issues/:id/ignore、POST /compliance/issues/:id/confirm-fail、POST /compliance/checks/:id/report、GET /compliance/rules、POST /compliance/rules、PATCH /compliance/rules/:id、DELETE /compliance/rules/:id。

## Cost / Approval / Notification / File

覆盖 x.md 第 14 节列出的成本、审批、通知、文件和 AI task 接口。所有 AI、OCR、解析、向量化、导出、合规和逐章生成接口返回 202 + task_id，并通过 SSE 加轮询兜底。
