### 1. 空安全（Null Safety）
- **问题**：未正确处理可空类型，导致潜在的`NullPointerException`。
- **检查点**：
  - 避免滥用`!!`（非空断言），优先使用安全调用`?.`或Elvis操作符`?:`。
  - 数据类或API返回的可空属性是否合理处理。
- **坏例子**：
  ```kotlin
  val length: Int = text!!.length // 风险：text可能为null
  ```
- **改进**：
  ```kotlin
  val length: Int = text?.length ?: 0 // 安全处理
  ```

#### 死代码
- 永远不会被执行到的代码块（如条件恒为 false 的分支、return 语句后的代码）
- 声明后从未被读取或引用的变
- 已注释掉的大段代码块（无明显保留意图的注释代码）

### 2. 函数与表达式简洁性
- **问题**：冗余代码破坏Kotlin的简洁性。
- **检查点**：
  - 单表达式函数是否用`=`简化（如`fun sum(a: Int, b: Int) = a + b`）。
  - 是否用`when`替代复杂`if-else`链。
  - 避免不必要的`return`（如Lambda中直接使用表达式结果）。
- **坏例子**：
  ```kotlin
  fun getGrade(score: Int): String {
      if (score >= 90) return "A"
      else if (score >= 80) return "B"
      else return "C"
  }
  ```
- **改进**：
  ```kotlin
  fun getGrade(score: Int) = when {
      score >= 90 -> "A"
      score >= 80 -> "B"
      else -> "C"
  }
  ```

### 3. 集合操作优化
- **问题**：低效的集合操作导致性能问题。
- **检查点**：
  - 优先使用`Sequence`处理大型集合（惰性求值减少中间对象）。
  - 避免重复操作（如多次`filter`合并为一次）。
  - 使用`groupBy`、`associate`等替代手动迭代。
- **坏例子**：
  ```kotlin
  val evenSquares = listOf(1, 2, 3).map { it * it }.filter { it % 2 == 0 } // 产生中间集合
  ```
- **改进**：
  ```kotlin
  val evenSquares = listOf(1, 2, 3).asSequence()
      .map { it * it }
      .filter { it % 2 == 0 }
      .toList() // 惰性计算
  ```

---

### 4. 协程（Coroutines）正确使用
- **问题**：协程泄漏或异常处理不当。
- **检查点**：
  - 使用结构化并发（`coroutineScope`或`supervisorScope`管理生命周期）。
  - 避免`GlobalScope`（易导致资源泄漏）。
  - 异常处理：用`try/catch`包裹`withContext`或`async`。
- **坏例子**：
  ```kotlin
  GlobalScope.launch { // 脱离作用域，可能泄漏
      fetchData()
  }
  ```
- **改进**：
  ```kotlin
  viewModelScope.launch { // 结构化并发
      try {
          withContext(Dispatchers.IO) { fetchData() }
      } catch (e: Exception) { /* 处理异常 */ }
  }
  ```

### 5. 类与对象设计
- **问题**：未合理使用Kotlin特性导致冗余。
- **检查点**：
  - **数据类**：纯数据对象用`data class`（自动生成`equals`/`hashCode`）。
  - **密封类/接口**：受限类型层次用`sealed class`（完善`when`分支检查）。
  - **委托**：属性委托（如`by lazy`）或类委托（`by`实现装饰器模式）。
- **坏例子**：
  ```kotlin
  class User(val name: String) {
      // 手动实现toString()/equals()...
  }
  ```
- **改进**：
  ```kotlin
  data class User(val name: String) // 自动生成标准方法
  ```

### 6. 资源管理与作用域函数
- **问题**：资源未释放或作用域函数滥用。
- **检查点**：
  - 文件/网络资源用`use`自动关闭（如`FileInputStream().use { ... }`）。
  - 作用域函数（`let`、`apply`等）需保持可读性，避免嵌套过深。
- **坏例子**：
  ```kotlin
  val file = File("path")
  val reader = BufferedReader(FileReader(file))
  // 忘记调用reader.close()
  ```
- **改进**：
  ```kotlin
  File("path").inputStream().use { stream ->
      // 自动关闭资源
  }
  ```

### 7. 性能陷阱
- **问题**：隐藏的性能开销。
- **检查点**：
  - 内联函数：高阶函数用`inline`减少Lambda开销（但避免对大函数内联）。
  - 常量：编译时常量用`const val`（取代`val`）。
  - 避免在循环中创建对象（如正则表达式`Regex`）。

### 8. 互操作性（Java交互）
- **问题**：Java调用Kotlin代码时的兼容性问题。
- **检查点**：
  - 暴露API时用`@JvmStatic`、`@JvmOverloads`优化Java调用。
  - 空安全注解：`@Nullable`/`@NonNull`辅助Java识别可空性。

### 9. 其他关键点
- **不可变性**：优先使用`val`而非`var`。
- **字符串处理**：用字符串模板（`"Value: $value"`）替代拼接。
