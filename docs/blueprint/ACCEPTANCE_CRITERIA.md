# 验收标准

完整平台雏形以 x.md 第 21 节 50 项为最终验收清单。本仓库实现时按 Loop 推进，每轮必须交付代码、测试或文档，并运行可用检查命令。

## 当前 Loop-0/1 验收

1. docs/input 文件完整。
2. docs/blueprint 文件完整。
3. frontend、backend、ai-service、infra、docker-compose.yml、.env.example 存在。
4. docker compose config 通过。
5. 前端可以 install / build。
6. Go 后端 go test ./... 通过。
7. Python AI 服务 compileall 通过。
8. backend /healthz 和 ai-service /healthz 可用。
