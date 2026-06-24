# Functional Options Pattern 规范

本规则定义 go-tools 项目中使用 Functional Options 模式的标准写法。
LLM 在编写新代码或重构现有代码时，必须遵循本规则。

## 1. 何时使用 Options 模式

满足以下**任一**条件时，必须使用 Options 模式：

- 构造函数有 **3 个及以上参数**
- 配置结构体有 **5 个及以上字段**
- 存在 `ApplyDefaults` / 默认值填充逻辑
- 参数可能在未来扩展（向前兼容需求）

## 2. 标准写法

### 2.1 Option 类型定义

```go
// Option 定义配置选项函数。
type Option func(*Target)
```

### 2.2 WithXxx 函数

每个可配置字段对应一个 `WithXxx` 函数：

```go
// WithExpiration 设置过期时间。
func WithExpiration(expiration time.Duration) Option {
    return func(c *Target) {
        if expiration > 0 {
            c.expiration = expiration
        }
    }
}

// WithPreKey 设置 key 前缀。空字符串忽略。
func WithPreKey(preKey string) Option {
    return func(c *Target) {
        if preKey != "" {
            c.preKey = preKey
        }
    }
}
```

**规则：**
- 函数名以 `With` 开头
- 每个函数只设置一个字段
- 对无效输入做防御（`> 0`、`!= ""`），不覆盖已有值
- 必须有 godoc 注释

### 2.3 构造函数

```go
// NewTarget 创建实例，支持 Options 配置。
// 默认配置：
//   - expiration: 5 分钟
//   - preKey: "DEFAULT_"
//   - capacity: 1024
func NewTarget(opts ...Option) *Target {
    t := &Target{
        expiration: defaultExpiration,
        preKey:     defaultPreKey,
        capacity:   defaultCapacity,
    }
    for _, opt := range opts {
        opt(t)
    }
    // 依赖 opts 的初始化放在后面
    t.internal = buildSomething(t.capacity)
    return t
}
```

**规则：**
- 构造函数签名：`NewXxx(opts ...Option) *Xxx`
- 先填默认值，再应用 opts
- 依赖 opts 结果的初始化放在 opts 应用之后
- godoc 注释列出默认值

### 2.4 向后兼容

旧构造函数保留，标记 `Deprecated`，内部委托给新构造函数：

```go
// Deprecated: 使用 NewTarget 配合 Options 替代。
func NewTargetWithConfig(name string, size int, timeout time.Duration) *Target {
    return NewTarget(
        WithName(name),
        WithCapacity(size),
        WithTimeout(timeout),
    )
}
```

## 3. 运行时更新

对于构造后可能需要修改的字段，提供 setter 方法：

```go
// SetExpiration 更新过期时间（并发安全）。
func (t *Target) SetExpiration(expiration time.Duration) {
    if expiration <= 0 {
        return
    }
    t.mu.Lock()
    defer t.mu.Unlock()
    t.expiration = expiration
}

// Update 批量更新配置（并发安全）。
func (t *Target) Update(opts ...Option) {
    t.mu.Lock()
    defer t.mu.Unlock()
    for _, opt := range opts {
        opt(t)
    }
}
```

## 4. 常量与默认值

集中定义在包级常量：

```go
const (
    defaultPreKey     = "CAPTCHA_"
    defaultExpiration = 5 * time.Minute
    defaultCapacity   = 1024
)
```

## 5. 禁止的写法

- ❌ 构造函数超过 3 个位置参数（不含 `opts ...Option`）
- ❌ 导出结构体字段后让调用方直接赋值（`store.Expiration = xxx`）
- ❌ 用 bool 开关替代 Options（如 `NewXxx(name string, enableFeature bool)`）
- ❌ 多个 `NewXxxWithYyy` 变体函数（应统一为一个 `NewXxx` + Options）

## 6. 并发安全

如果结构体字段在构造后可通过 setter 修改：
- 使用 `sync.RWMutex` 保护可变字段
- Getter 用 `RLock`，Setter 用 `Lock`
- 内部方法用 `snapshot()` 一次性读取多个字段，避免中间态

底层依赖库自带并发安全时（如 `samber/hot`），不需要对外层操作重复加锁。
