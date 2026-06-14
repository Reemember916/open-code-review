# 嵌入式 C 自动审查试用指南

本文面向源码仓库试用版用户，目标是在不理解底层规则细节的情况下完成一次嵌入式 C 项目自动审查。

## 1. 环境准备

必须安装：

- Go，用于从源码运行：`go version`
- cppcheck，用于 C/C++ 静态分析：`cppcheck --version`

可选安装：

- LLM 配置，仅在使用 `--ai` 时需要

进入仓库后先运行：

```powershell
go run .\cmd\opencodereview doctor
```

`cppcheck` 必须是 `OK`。LLM 如果是 `WARN`，普通静态扫描仍可使用。

## 2. 一键审查

普通用户只需要记住：

```powershell
go run .\cmd\opencodereview scan C:\Projects\YourEmbeddedProject
```

工具会自动发现：

- `.opencodereview/project.json`
- `.opencodereview/roles.json`
- `.opencodereview/baseline.json`
- `compile_commands.json`
- `build/compile_commands.json`
- `Src` / `src` / `Source`
- `Include` / `include` / `Inc`

## 3. 生成报告

生成 HTML 报告：

```powershell
go run .\cmd\opencodereview scan --format html C:\Projects\YourEmbeddedProject > report.html
```

生成 SARIF 报告：

```powershell
go run .\cmd\opencodereview scan --format sarif C:\Projects\YourEmbeddedProject > report.sarif
```

## 4. AI 解释

AI 默认不开启。需要先配置 LLM：

```powershell
go run .\cmd\opencodereview config set llm.url https://your-llm-endpoint
go run .\cmd\opencodereview config set llm.auth_token your-token
go run .\cmd\opencodereview config set llm.model your-model
go run .\cmd\opencodereview llm test
```

开启 AI finding 解释：

```powershell
go run .\cmd\opencodereview scan --ai --format html C:\Projects\YourEmbeddedProject > report-ai.html
```

默认只解释 `blocking` 和 `review` finding，最多 20 条。可以调整：

```powershell
go run .\cmd\opencodereview scan --ai --ai-limit 5 C:\Projects\YourEmbeddedProject
```

AI 只提供解释、风险和修复建议，不改变静态扫描结论、门禁结果和 baseline。

## 5. 历史问题 baseline

首次接入旧项目时，可以先固化历史 finding：

```powershell
go run .\cmd\opencodereview rules scan --write-baseline .opencodereview\baseline.json Src
```

之后扫描时历史项会标记为 `suppressed`，不触发门禁。

## 6. CI 门禁

只要有 `blocking` 或 `review` finding 就失败：

```powershell
go run .\cmd\opencodereview scan --fail-on review C:\Projects\YourEmbeddedProject
```

只阻断 `blocking`：

```powershell
go run .\cmd\opencodereview scan --fail-on blocking C:\Projects\YourEmbeddedProject
```

## 7. 源码内豁免

明确接受的例外可以写在源码旁边：

```c
// ocr-disable-next-line embedded.memory.no_strcpy
strcpy(dst, src);
```

或：

```c
sprintf(buf, "%d", x); // ocr-disable-line
```

也支持规则通配：

```c
// ocr-disable-next-line cppcheck.*
```

被豁免的 finding 仍会保留在报告中，但 disposition 会变为 `suppressed`。

## 8. 推荐试用流程

```powershell
go test ./...
go run .\cmd\opencodereview doctor
go run .\cmd\opencodereview scan C:\Projects\YourEmbeddedProject
go run .\cmd\opencodereview scan --format html C:\Projects\YourEmbeddedProject > report.html
go run .\cmd\opencodereview scan --ai --ai-limit 5 --format html C:\Projects\YourEmbeddedProject > report-ai.html
```

