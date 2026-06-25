# 工具类提取设计：ncgo → go-tools

> 日期：2026-06-25
> 状态：已批准，待实施
> 范围：将 ncgo 中 3 个通用工具模块提取到 go-common，作为独立库供 ncgo 和其他项目复用

---

## 1. 背景

### 1.1 问题

ncgo 是一个脚手架 CLI，用于生成 Go 微服务项目。它的 `internal/` 下嵌入了多个通用工具模块（AST 操作、命令执行、模板辅助），这些模块以模板代码的形式被复制到每个生成的项目中。

这导致：
- **重复**：相同逻辑在 ncgo 和每个生成项目中各存一份
- **不一致**：工具逻辑的修复/增强无法自动同步到所有项目
- **不可复用**：其他项目无法使用这些工具

### 1.2 目标

将 ncgo 中的 3 个通用工具模块提取到 `go-common`，作为独立包发布：

| 工具 | 目标包 | 当前状态 |
|------|--------|---------|
| AST 代码操作引擎 | `go-common/astutil/` + `go-common/astutil/gen/` | 不存在 |
| 命令执行包装器 | `go-common/executil/` | 不存在 |
| 模板辅助函数 | `go-common/templateutil/` | 不存在 |

### 1.3 非目标

- 不提取 ncgo 的脚手架逻辑（`internal/scaffold/`）
- 不提取 ncgo 的特定功能（`protolint`、`k8s` 生成等）
- 不改变 ncgo 的 CLI 接口

---

## 2. 架构决策

### 2.1 包组织

**决策**：独立 `xxxutil` 包（方案 B）

```
go-common/
├── astutil/          ← AST 代码操作
│   └── gen/          ← 代码生成（jennifer 风格）
├── executil/         ← 命令执行
└── templateutil/     ← 模板辅助
```

**理由**：
- 与 go-common 现有命名约定一致（`netutil`、`timeutil`）
- 每个包职责单一，按需引入
- 非代码生成场景也可使用（如 `executil` 适用于任何需要执行外部命令的场景）

### 2.2 AST 库选择

**决策**：基于 `dave/dst`（Decorated Syntax Tree）封装

**备选方案对比**：

| 方案 | 优点 | 缺点 |
|------|------|------|
| A. 基于 `dave/dst` | 格式化完美保持；API 简洁 | 多一个外部依赖 |
| B. 基于标准 `go/ast` | 零额外依赖 | 复杂重构时格式化可能丢失 |
| C. 混合（dst + jennifer） | 覆盖所有场景 | 最复杂 |

**理由**：
- `dst` 将注释/空白直接附加到节点，解决了 `go/ast` 的核心痛点
- ncgo 的模板代码修改需要输出人类可读的代码，格式化保持是关键
- `dst` 已是成熟方案（540+ stars），不需要从零造轮子
- 如果未来需要从零生成代码，可以用 `astutil/gen` 子包（jennifer 风格）

### 2.3 API 设计策略

**决策**：重新设计（非直接迁移 ncgo 代码）

**理由**：
- 趁提取机会审视 API，采用更好的抽象
- ncgo 的现有实现可作为参考，但不受其约束
- go-tools 版本的 API 应面向更广泛的使用场景

---

## 3. 详细设计

### 3.1 `go-common/astutil/`

#### 定位
通用 Go AST 操作库。基于 `dave/dst` 封装，提供声明式、可组合的 API，用于解析、查询、修改、生成 Go 代码。

#### 包结构

```
go-common/astutil/
├── astutil.go       ← ParseFile, ParseSource, File 类型
├── query.go         ← Find* 查询方法
├── ops.go           ← 内置操作（AddImport, Insert, Replace 等）
├── format.go        ← Format, WriteTo 输出方法
└── gen/
    ├── gen.go       ← jennifer 风格流式 API，从零生成代码
    ├── stmt.go      ← 语句构建器
    └── expr.go      ← 表达式构建器
```

#### 核心 API

**解析与修改（`astutil/`）**

```go
// 解析：源文件 → dst 装饰树
file, err := astutil.ParseFile("main.go")
file, err := astutil.ParseSource(src []byte)

// 查询：查找节点
funcs := file.FindFunctions()                    // 所有函数
fn := file.FindFunction("HandleRequest")         // 按名称
decls := file.FindDecls(&dst.GenDecl{})          // 按类型
imports := file.FindImports()                    // 所有 import

// 修改：声明式操作（保持格式化）
file.Apply(
    astutil.AddImport("fmt"),
    astutil.AddImport("context"),
    astutil.RemoveImport("os"),
    astutil.EnsureFunction("init", bodyStmts),     // bodyStmts: []dst.Stmt
    astutil.InsertAfter("//inject:here", newStmt), // newStmt: dst.Stmt，基于注释标记定位
    astutil.ReplaceNode(oldNode, newNode),         // oldNode, newNode: dst.Node
)

// 输出：格式化源码
out, err := file.Format()         // 格式化输出
err = file.WriteTo("main.go")     // 写回文件
```

**从零生成（`astutil/gen/`）**

```go
import "github.com/byx-darwin/go-tools/go-common/astutil/gen"

f := gen.NewFile("mypackage")

// 添加 import
f.Import("context")
f.Import("fmt")

// 生成函数
f.Func("HandleRequest").
    Params(gen.Param("ctx", "context.Context"), gen.Param("name", "string")).
    Results(gen.Result("error")).
    Body(
        gen.Statement().If(gen.Call("name", "!=").Lit("")).
            Block(gen.Return(gen.Nil())),
        gen.Return(gen.Call("fmt.Errorf").Lit("empty name")),
    )

// 生成结构体
f.Struct("Config").
    Field("Name", "string").Tag(`json:"name"`).
    Field("Timeout", "time.Duration").Tag(`json:"timeout"`)

// 输出
code, _ := f.Render()  // → 格式化的 Go 源码
```

#### 两个子包的分工

| 场景 | 用哪个 | 说明 |
|------|--------|------|
| 修改现有 Go 文件 | `astutil/` | 基于 dst，完美保持格式 |
| 从零生成新文件 | `astutil/gen/` | jennifer 风格流式 API |
| 混合场景 | 两者配合 | `gen` 生成代码片段 → `astutil` 插入到已有文件 |

#### 关键特性

| 特性 | 说明 |
|------|------|
| **幂等操作** | `AddImport` 检测到已存在时跳过，不会重复 |
| **标记定位** | 通过注释标记（如 `//inject:here`）精确定位插入点 |
| **操作可组合** | `Apply` 接受多个操作，按顺序执行 |
| **错误收集** | 多个操作的错误聚合返回，不会因一个失败中断全部 |
| **格式化保持** | 基于 dst，修改后输出保持原始格式 |

#### 依赖

- `github.com/dave/dst` — 装饰语法树（~540 stars，活跃维护）

---

### 3.2 `go-common/executil/`

#### 定位
增强的命令执行包装器。封装 `os/exec`，提供上下文取消、超时控制、流式输出、错误分类、可 mock 接口。

#### 核心 API

```go
// Runner 可 mock 的执行接口
type Runner interface {
    Run(ctx context.Context, cmd *Cmd) *Result
}

// Cmd 命令配置
type Cmd struct {
    Name     string            // 命令名（如 "go", "npm"）
    Args     []string          // 参数
    Dir      string            // 工作目录
    Env      []string          // 环境变量
    Stdin    io.Reader         // 标准输入
    Timeout  time.Duration     // 超时（0 表示无超时）
    OnStdout func([]byte)      // 流式输出回调（可选）
    OnStderr func([]byte)      // 流式错误回调（可选）
}

// Result 执行结果
type Result struct {
    Stdout   []byte          // 标准输出
    Stderr   []byte          // 标准错误
    ExitCode int             // 退出码
    Err      error           // 执行错误（分类错误）
}
```

#### 错误分类

```go
// NotFoundError 命令不存在
type NotFoundError struct {
    Name    string           // 命令名
    Hint    string           // 安装提示（如 "run: brew install go"）
}

// ExitError 命令退出码非零
type ExitError struct {
    ExitCode int
    Stderr   []byte          // 截断的 stderr（最多 1KB）
}

// TimeoutError 命令执行超时
type TimeoutError struct {
    Duration time.Duration
}
```

#### 使用示例

```go
runner := executil.New()

// 基础用法
result := runner.Run(ctx, &executil.Cmd{
    Name: "go",
    Args: []string{"build", "./..."},
    Dir:  "/path/to/project",
})
if result.Err != nil {
    var nfe *executil.NotFoundError
    if errors.As(result.Err, &nfe) {
        fmt.Println("命令不存在:", nfe.Hint)
    }
}

// 带超时 + 流式输出
result := runner.Run(ctx, &executil.Cmd{
    Name:    "npm",
    Args:    []string{"install"},
    Timeout: 30 * time.Second,
    OnStdout: func(line []byte) {
        fmt.Print(string(line))  // 实时输出
    },
})
```

#### 关键特性

| 特性 | 说明 |
|------|------|
| **上下文取消** | 支持 `ctx.Done()` 取消正在执行的命令 |
| **超时控制** | `Cmd.Timeout` 自动包裹 `context.WithTimeout` |
| **流式输出** | `OnStdout` / `OnStderr` 回调实时输出 |
| **错误分类** | `NotFoundError`（含安装提示）、`ExitError`（含 stderr）、`TimeoutError` |
| **可 mock** | `Runner` 接口便于单元测试 |
| **输出截断** | `ExitError.Stderr` 最多保留 1KB，避免内存爆炸 |

#### 依赖

- 无外部依赖（仅标准库 `os/exec`、`context`）

---

### 3.3 `go-common/templateutil/`

#### 定位
可插拔的模板辅助函数库。提供 `text/template` 的函数扩展，支持默认函数集 + 自定义注册。

#### 核心 API

```go
// FuncMap 返回默认函数集
func FuncMap() template.FuncMap

// Registry 可插拔的函数注册器
type Registry struct { ... }

// NewRegistry 创建空注册器
func NewRegistry() *Registry

// Register 注册单个函数
func (r *Registry) Register(name string, fn any) *Registry

// RegisterAll 批量注册
func (r *Registry) RegisterAll(funcs template.FuncMap) *Registry

// FuncMap 返回已注册的所有函数
func (r *Registry) FuncMap() template.FuncMap

// Default 返回内置默认函数集
func (r *Registry) Default() *Registry

// Render 便捷函数：使用默认函数集渲染模板
func Render(tmpl string, data any) (string, error)

// RenderWith 使用自定义 Registry 渲染
func RenderWith(tmpl string, data any, reg *Registry) (string, error)
```

#### 默认函数集

| 分类 | 函数 | 说明 |
|------|------|------|
| **字符串** | `ToLower`, `ToUpper` | 大小写转换 |
| **命名** | `LowerFirst`, `UpperFirst` | 首字母大小写 |
| **命名** | `ToCamel`, `ToSnake`, `ToKebab` | 命名风格转换 |
| **命名** | `ExportName` | 生成导出名（首字母大写） |
| **命名** | `PrivateName` | 生成非导出名（首字母小写） |
| **复数** | `Plural`, `Singular` | 英文单复数（代码生成常用） |

#### 使用示例

```go
// 方式 1：便捷函数（使用默认函数集）
out, err := templateutil.Render("Hello {{ .Name | ToLower }}", data)

// 方式 2：自定义注册
reg := templateutil.NewRegistry().
    Default().                                    // 内置函数
    Register("dateFormat", func(t time.Time) string {
        return t.Format("2006-01-02")
    }).
    Register("json", func(v any) string {
        b, _ := json.Marshal(v)
        return string(b)
    })

tmpl := template.Must(template.New("t").Funcs(reg.FuncMap()).Parse(tmplStr))
```

#### 关键特性

| 特性 | 说明 |
|------|------|
| **可插拔** | `Registry` 支持自定义函数注册 |
| **链式 API** | `Register` 返回 `*Registry`，支持链式调用 |
| **默认集** | `Default()` 提供代码生成常用的命名转换函数 |
| **便捷入口** | `Render` 函数快速渲染，无需手动创建 `Registry` |

#### 依赖

- 无外部依赖（仅标准库 `text/template`）

---

## 4. 与 ncgo 的集成

### 4.1 迁移路径

提取完成后，ncgo 需要：

1. **添加依赖**：
   ```bash
   go get github.com/byx-darwin/go-tools/go-common
   ```

2. **替换内部实现**：
   - `internal/scaffold/infra/astwire/` → `go-common/astutil/`
   - `internal/exec/` → `go-common/executil/`
   - `internal/scaffold/template/render.go` → `go-common/templateutil/`

3. **更新模板**：
   - ncgo 生成的项目代码中，如果有使用这些工具，应改为引用 go-tools 包，而不是复制粘贴

### 4.2 向后兼容

- go-tools 的新包不依赖 ncgo，可独立发布
- ncgo 迁移后，旧版本仍可工作（内部实现未变）
- 新项目可直接使用 go-tools，无需 ncgo

---

## 5. 测试策略

### 5.1 单元测试

每个包需要覆盖：

| 包 | 测试重点 |
|---|---------|
| `astutil/` | 解析、查询、修改操作的正确性；格式化保持；幂等性 |
| `astutil/gen/` | 代码生成的正确性；输出格式化 |
| `executil/` | 命令执行、超时、取消、错误分类、流式输出 |
| `templateutil/` | 默认函数集的正确性；自定义注册；链式 API |

### 5.2 集成测试

- **astutil + ncgo 场景**：用 ncgo 的实际模板代码测试 `astutil` 的修改操作
- **executil + 真实命令**：测试 `go build`、`npm install` 等真实场景
- **templateutil + 模板**：测试 ncgo 模板中使用的命名转换函数

### 5.3 验证命令

```bash
# 构建
go build ./go-common/astutil/... ./go-common/executil/... ./go-common/templateutil/...

# 测试
go test ./go-common/astutil/... ./go-common/executil/... ./go-common/templateutil/... -count=1

# Lint
golangci-lint run ./go-common/astutil/... ./go-common/executil/... ./go-common/templateutil/...
```

---

## 6. 实施阶段

### 阶段 1：`templateutil`（最简单，无外部依赖）

1. 创建 `go-common/templateutil/` 包
2. 实现 `Registry`、默认函数集、`Render` 便捷函数
3. 编写单元测试
4. 通过 lint 和测试

### 阶段 2：`executil`（无外部依赖，但逻辑较复杂）

1. 创建 `go-common/executil/` 包
2. 实现 `Runner` 接口、`Cmd`/`Result` 类型、错误分类
3. 实现上下文取消、超时、流式输出
4. 编写单元测试（使用 mock `Runner`）
5. 通过 lint 和测试

### 阶段 3：`astutil/`（基于 dst，最复杂）

1. 添加 `github.com/dave/dst` 依赖
2. 创建 `go-common/astutil/` 包
3. 实现解析、查询、修改操作
4. 编写单元测试
5. 通过 lint 和测试

### 阶段 4：`astutil/gen/`（jennifer 风格代码生成）

1. 创建 `go-common/astutil/gen/` 子包
2. 实现流式 API（`File`、`Func`、`Struct` 等构建器）
3. 编写单元测试
4. 通过 lint 和测试

### 阶段 5：ncgo 集成（可选，后续进行）

1. ncgo 添加 go-common 依赖
2. 替换内部实现为 go-tools 包
3. 更新模板引用
4. 端到端测试

---

## 7. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| `dave/dst` 依赖引入兼容性问题 | 中 | 选择稳定版本；仅在 `astutil` 中使用，不影响其他包 |
| AST 操作 API 设计不够灵活 | 中 | 参考 ncgo 实际用例；预留扩展点 |
| ncgo 迁移时 API 不匹配 | 低 | 先提取，后迁移；ncgo 可暂不迁移 |
| 格式化保持不如预期 | 低 | 充分测试；`dst` 已是成熟方案 |

---

## 8. 成功标准

- [ ] 4 个包（`astutil`、`astutil/gen`、`executil`、`templateutil`）全部实现
- [ ] 所有包通过单元测试、lint、构建验证
- [ ] `astutil` 能正确处理 ncgo 的实际模板代码修改场景
- [ ] `executil` 支持上下文取消、超时、流式输出、错误分类
- [ ] `templateutil` 支持默认函数集 + 自定义注册
- [ ] 文档完善（godoc 注释、使用示例）

---

## 9. 参考资料

- [golang.org/x/tools/go/ast/astutil](https://pkg.go.dev/golang.org/x/tools/go/ast/astutil) — 官方 AST 工具库
- [dave/dst](https://github.com/dave/dst) — Decorated Syntax Tree（~540 stars）
- [dave/jennifer](https://github.com/dave/jennifer) — Go 代码生成器
- [Rewriting Go source code with AST tooling](https://eli.thegreenplace.net/2021/rewriting-go-source-code-with-ast-tooling/) — Eli Bendersky 的 AST 重写教程
- ncgo 现有实现：
  - `internal/scaffold/infra/astwire/astwire.go` — AST 操作
  - `internal/exec/exec.go` — 命令执行
  - `internal/scaffold/template/render.go` — 模板辅助
