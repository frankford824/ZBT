# 文档导出设计

最小 docx 链路：

Tiptap JSON -> 中间文档结构 -> docxtpl / python-docx -> docx -> MinIO 保存 -> bid_exports 记录 -> 前端下载。

增强链路：

1. 排版模板。
2. 页眉页脚。
3. 封面。
4. 水印。
5. 目录。
6. PDF 转换。
7. 分离标书技术标 / 商务标分别导出。
8. ZIP 打包。

原则：前端编辑器只负责内容结构，不做网页端 Word 级分页所见即所得。
