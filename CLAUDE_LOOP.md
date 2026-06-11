# 智标通 Claude / AI Agent Loop 协议

每轮执行：

1. 阅读 docs/blueprint。
2. 阅读 docs/blueprint/DEV_LOOP_LOG.md。
3. 判断当前完成状态。
4. 写入本轮目标。
5. 实现代码、测试或文档。
6. 运行检查命令。
7. 修复错误。
8. 更新 DEV_LOOP_LOG.md。
9. 输出本轮总结。

禁止只写文档、只做静态页面、删除 8 大模块、少于 V2.4 的 14 页面、全 Python 单体化、绕过 RLS、绕过 ModelRouter 或硬编码 API Key。
