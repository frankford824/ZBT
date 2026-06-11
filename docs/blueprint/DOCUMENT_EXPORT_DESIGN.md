# 文档导出设计

最小 docx 链路：

Tiptap JSON -> 中间文档结构 -> docxtpl / python-docx -> docx -> MinIO 保存 -> bid_exports 记录 -> 前端下载。

当前实现状态：已完成分离标书技术标 / 商务标的最小 docx 导出闭环，并支持投标文件全套 ZIP 打包。Go 创建 bid_exports、ai_tasks 和待确认 file_asset，Python `/tasks/export/docx` 或 `/tasks/export/zip` 后台生成文件并上传 MinIO，HMAC 回调后 Go 更新导出状态，前端第 7 步可创建导出任务并下载完成文件。

增强链路：

1. 排版模板。
2. 页眉页脚。
3. 封面。
4. 水印。
5. 目录。
6. PDF 转换。
7. 分离标书技术标 / 商务标分别导出。
8. ZIP 内文件清单、命名规则和电子标格式增强。

原则：前端编辑器只负责内容结构，不做网页端 Word 级分页所见即所得。
