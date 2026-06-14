#### 明确的错别字或拼写错误识别
- 定义处的变量名、常量名、函数名中包含拼写错误，请不要报告调用处的拼写错误
- 日志或异常信息中的字符串包含影响阅读理解的拼写错误

#### malloc/free配对使用
**检查要点：**
- 每个`malloc()`都有对应的`free()`
- 避免重复释放同一内存块
- 释放后将指针设为NULL

**示例：**
```c
// ❌ 错误示例
char* buffer = malloc(1024);
// 使用buffer...
// 忘记释放内存

// ✅ 正确示例
char* buffer = malloc(1024);
if (buffer != NULL) {
    // 使用buffer...
    free(buffer);
    buffer = NULL;
}
```

#### 内存泄漏检查
**检查要点：**
- 所有分配的内存在函数退出前都被释放
- 错误处理路径中也要释放内存
- 使用工具如Valgrind进行检测

#### 缓冲区溢出防护
**检查要点：**
- 数组访问前检查边界
- 字符串操作使用安全函数
- 循环边界条件正确

**示例：**
```c
// ❌ 危险操作
char buffer[100];
strcpy(buffer, user_input); // 可能溢出

// ✅ 安全操作
char buffer[100];
strncpy(buffer, user_input, sizeof(buffer) - 1);
buffer[sizeof(buffer) - 1] = '\0';
```

#### 字符串操作安全
**推荐使用的安全函数：**
- `strncpy()` 替代 `strcpy()`
- `strncat()` 替代 `strcat()`
- `snprintf()` 替代 `sprintf()`

#### 命名规范
**要求：**
- 使用snake_case命名方式
- 变量名要有意义
- 常量使用UPPER_CASE
