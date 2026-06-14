# 嵌入式 C 智能审查工具项目蓝图

## 1. 项目定位

本项目的目标不是做“静态分析工具集合器”，而是构建一个面向嵌入式 C 项目的规则驱动审查系统。

核心定位：

```text
以嵌入式 C 规则库为核心，
以多种分析后端为执行手段，
以 AI 为上下文解释和误报复核层，
最终输出适合开发者和 CI/MR 流程消费的审查意见。
```

## 2. 技术路线转型

原始路线：

```text
集成 cppcheck / semgrep / clang-tidy / LDRA / flawfinder
  -> 合并工具结果
  -> AI 解释
```

调整后的路线：

```text
定义统一规则模型
  -> 构建嵌入式 C 规则库
  -> 为不同规则选择合适执行后端
  -> 产出结构化证据
  -> AI 复核、降噪、解释
  -> 输出行级审查意见
```

核心转变：

```text
工具不是核心资产，规则模型和嵌入式规则库才是核心资产。
```

进一步明确：

```text
通用 C/C++ 静态分析能力优先复用成熟开源工具，
例如 cppcheck、clang-tidy、Clang Static Analyzer、CodeQL、semgrep。

本项目不在早期从零自研完整 C parser、AST、CFG、数据流分析和类型系统。
本项目优先建设：
1. 统一规则模型
2. 嵌入式 C 项目语义规则
3. 多后端适配
4. finding/evidence 归一化
5. diff/role/context 过滤
6. AI 复核和解释层
```

## 3. 总体架构

```text
Git diff / 指定文件
  -> Code Model Builder
  -> Fact Extractor
  -> Rule Engine
  -> Finding Normalizer
  -> AI Review Layer
  -> Report / MR Comment / JSON Output
```

模块划分：

```text
internal/ruleengine/
  rule.go              # 规则模型
  fact.go              # 代码事实模型
  finding.go           # 统一问题模型
  evidence.go          # 证据模型
  engine.go            # 规则调度
  suppress.go          # 豁免和降噪

internal/ruleengine/backends/
  regex/               # 文本/模式规则
  call/                # 函数调用事实规则
  cppcheck/            # cppcheck 结果适配
  semgrep/             # semgrep 结果适配
  clang/               # clang-tidy / Clang Static Analyzer 适配
  codeql/              # CodeQL 查询结果适配
  external_report/     # 商业工具报告导入

internal/rulesets/
  embedded_c/          # 嵌入式 C 规则库
  misra_like/          # MISRA-inspired 规则，不等同正式 MISRA 合规
  project/             # 项目自定义规则
```

## 4. 规则模型

一条规则应包含以下要素：

```text
1. id
2. title
3. severity
4. scope
5. match 条件
6. context 条件
7. evidence 要求
8. message
9. suggestion
10. suppression 条件
```

示例：

```json
{
  "id": "embedded.isr.no_blocking_call",
  "title": "ISR 中禁止调用阻塞函数",
  "severity": "high",
  "scope": "function",
  "where": {
    "function_role": "isr"
  },
  "match": {
    "calls_any": ["DelayMs", "SemaphorePend", "QueueRecvBlocking"]
  },
  "message": "ISR 中调用阻塞 API，可能导致实时性丢失",
  "suggestion": "改为置标志位，在任务或主循环上下文处理"
}
```

## 5. 静态规则的本质

静态规则不是某个工具的输出，而是在代码模型上的可验证约束：

```text
代码 -> 抽象模型 -> 提取事实 -> 应用规则 -> 产出证据
```

规则执行结果必须能够回答：

```text
什么规则命中了？
在哪个文件、哪一行？
基于什么代码事实？
为什么这在嵌入式 C 场景中有风险？
是否需要 AI 或人工复核？
```

## 6. 规则分层

### L1 文本/模式规则

适合：

```text
危险函数
禁用 API
简单宏规则
命名规则
文件路径规则
```

执行后端：

```text
regex
semgrep
```

### L2 AST 规则

适合：

```text
函数调用
赋值语句
条件表达式
循环语句
switch/default
宏调用结构
```

执行后端：

```text
tree-sitter-c
clang AST
```

### L3 类型规则

适合：

```text
signed/unsigned 混用
窄化转换
volatile 丢失
const 丢失
浮点和整数转换
```

执行后端：

```text
clang-tidy
Clang AST/diagnostics
compile_commands.json
```

### L4 控制流/数据流规则

适合：

```text
未初始化使用
空指针解引用
错误路径资源释放
数组边界检查
资源生命周期
```

执行后端：

```text
cppcheck
clang static analyzer
CodeQL
必要时再补充自研轻量 def-use 分析
```

### L5 项目语义规则

适合：

```text
ISR 实时性
DMA buffer 生命周期
寄存器访问约束
状态机转移
故障处理流程
双通道投票逻辑
安全状态切换
```

执行后端：

```text
项目配置
自定义规则
AI 辅助复核
```

## 7. AI 的职责边界

AI 不负责直接“猜问题”。AI 的职责是消费规则引擎产出的 evidence。

规则引擎负责：

```text
命中规则
定位代码
生成证据
给出初始严重度
```

AI 负责：

```text
结合上下文判断误报
解释嵌入式风险
合并重复 finding
生成开发者可读的审查意见
给出修复建议
```

AI 输出建议分为：

```text
confirmed     # 证据充分，输出行级评论
needs_review  # 需要人工确认
ignored       # 明显误报，不输出
```

## 8. 商业工具边界

LDRA、Polyspace、Coverity、Klocwork 等商业工具不能作为内置后端复制或复刻。

允许方式：

```text
读取用户已授权工具生成的报告
解析 XML / JSON / CSV / text 结果
转成统一 Finding
参与 AI 解释和报告合并
```

不允许方式：

```text
内置商业工具分析引擎
复制商业工具规则库
绕过商业工具授权
分发商业工具专有能力
```

因此商业工具只作为：

```text
external report adapter
```

而不是核心后端。

## 8.1 开源工具后端边界

开源静态分析工具不作为产品核心资产，但可以作为规则执行后端。

推荐后端职责：

```text
cppcheck
  通用 C/C++ 缺陷、未初始化、未使用变量、重复赋值、数组越界、部分 MISRA addon

clang-tidy / Clang Static Analyzer
  类型信息、cast、signed/unsigned、const/volatile、clang-analyzer-* 路径分析

CodeQL
  更复杂的数据流、安全查询、buffer overflow、uninitialized local 等

semgrep
  项目 API 禁用规则、简单模式规则、快速规则验证
```

本项目对这些工具的使用方式：

```text
Rule -> Backend -> Raw result -> Normalized Finding/Evidence -> Context filter -> AI review
```

注意：

```text
不是工具报什么就全部输出；
而是规则模型决定需要什么能力，再选择合适后端执行或导入结果。
```

## 9. MVP 范围

第一阶段只做能验证路线的最小闭环。

目标：

```text
指定文件或 Git diff
  -> 运行内置规则
  -> 产出 Finding
  -> AI 复核和解释
  -> 输出 JSON / 文本审查结果
```

优先实现：

```text
1. Rule JSON schema
2. Finding / Evidence 模型
3. diff 相关行过滤
4. regex backend
5. call backend
6. 简单函数和调用提取
7. 函数角色识别和角色配置
8. 嵌入式 C 初始规则库
9. rules scan CLI
10. AI prompt 注入
```

暂不追求：

```text
完整 MISRA 合规
从零自研完整 AST 类型系统
从零自研完整数据流分析
商业工具深度集成
GUI
```

## 10. 初始规则库建议

### 实时性规则

```text
embedded.isr.no_blocking_call
embedded.isr.no_dynamic_allocation
embedded.loop.require_timeout
embedded.loop.require_watchdog_or_exit
```

### 内存和指针规则

```text
embedded.memory.no_strcpy
embedded.memory.no_sprintf
embedded.pointer.null_check_before_deref
embedded.buffer.bound_check_before_copy
```

### 寄存器和 volatile 规则

```text
embedded.register.use_access_macro
embedded.register.no_read_modify_write_on_status_clear
embedded.volatile.shared_state_requires_volatile
```

### 状态机和故障处理规则

```text
embedded.state.switch_requires_default
embedded.state.transition_requires_error_path
embedded.fault.clear_requires_record
embedded.safety.shutdown_requires_pwm_disable
```

### MISRA-inspired 规则

```text
misra_like.no_implicit_narrowing
misra_like.no_assignment_in_condition
misra_like.macro_args_parenthesized
misra_like.non_void_all_paths_return
misra_like.no_unused_return_value
```

说明：MISRA-inspired 规则只表示受 MISRA 思想启发，不代表正式 MISRA 合规结论。

## 11. 开发阶段规划

### 阶段 1：规则模型和最小引擎

交付：

```text
Rule JSON schema
Finding / Evidence
regex backend
diff/file 输入
JSON 输出
基础测试
```

### 阶段 2：嵌入式 C 初始规则库

交付：

```text
ISR 规则
超时规则
危险函数规则
switch/default 规则
简单寄存器访问规则
```

### 阶段 3：AI 复核层

交付：

```text
finding prompt 注入
confirmed / needs_review / ignored 分类
行级审查评论
误报降噪
```

### 阶段 4：AST 和外部后端

交付：

```text
cppcheck 结果适配
semgrep 结果适配
clang-tidy / Clang Static Analyzer 适配
CodeQL 查询结果适配
商业工具报告导入适配
必要时再引入 tree-sitter 或 clang AST 直接事实提取
```

### 阶段 5：CI 和工程化

交付：

```text
GitHub Actions / GitLab CI
JSON/SARIF 输出
规则豁免
规则级别配置
审查报告摘要
```

## 12. 当前 Open Code Review 代码利用方式

可继续复用：

```text
Git diff 解析
文件过滤
LLM 调用
工具调用机制
行级评论定位
JSON/text 输出
session viewer
```

需要调整：

```text
从 Analyzer 驱动转为 Rule 驱动
从工具结果聚合转为规则 evidence 聚合
把 external analyzer 变成规则后端之一
```

当前已新增的 `internal/staticanalysis` 可作为过渡实现，后续建议逐步迁移为：

```text
internal/ruleengine
```

## 13. 成功标准

短期成功：

```text
能对指定嵌入式 C 文件命中几类明确规则
finding 有文件、行号、规则编号、证据
AI 能根据 evidence 给出清晰审查意见
```

中期成功：

```text
规则库可配置
误报可抑制
CI 可运行
MR/PR 可输出行级评论
```

长期成功：

```text
形成嵌入式 C 专用规则资产
支持项目语义规则
支持商业工具报告导入
具备内部商用级审查体验
```
