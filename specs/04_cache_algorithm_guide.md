# 缓存算法选择指南

> 日期：2026-06-23
> 状态：Confirmed — 底层库切换为 `github.com/samber/hot`

## 一、背景

go-tools 原有的 `tools/cache/core/` 包含 5 种自定义缓存实现（FIFO/LRU/LFU/CLOCK/MRU），底层使用 `github.com/Code-Hex/go-generics-cache`。

**决策**：统一切换为 `github.com/samber/hot`（ncgo 模板已验证），删除自定义实现。

## 二、缓存算法对比

| 算法 | 淘汰策略 | 时间复杂度 | 内存开销 | 适用场景 |
|------|---------|-----------|---------|---------|
| **FIFO** | 先进先出，按写入顺序淘汰 | O(1) | 低 | 数据访问模式均匀，无热点 |
| **LRU** | 最近最少使用，淘汰最久未访问的 | O(1) | 中（需维护双向链表） | 存在时间局部性，近期访问的数据可能再次访问 |
| **LFU** | 最少频繁使用，淘汰访问次数最少的 | O(log n) | 高（需维护频率计数） | 存在明显的热点数据，访问频率差异大 |
| **CLOCK** | LRU 的近似算法，使用访问位 + 时钟指针 | O(1) | 低（无需链表） | 需要 LRU 效果但内存受限 |
| **MRU** | 最近最常使用，淘汰最近访问的 | O(1) | 中 | 访问模式呈周期性，刚访问的数据短期内不会再次访问 |

## 三、算法详解与使用场景

### 3.1 FIFO（First In First Out）

**原理**：按写入顺序维护队列，容量满时淘汰最早写入的条目。

**优点**：
- 实现简单，O(1) 操作
- 无需跟踪访问模式，内存开销最小

**缺点**：
- 无法区分热点数据，可能淘汰仍频繁使用的旧数据

**适用场景**：
- 数据访问模式均匀分布，无明显热点
- 简单的任务队列、事件缓冲区
- 日志消息暂存

**samber/hot 用法**：
```go
cache := hot.NewHotCache[string, Event](hot.FIFO, 1000).Build()
```

### 3.2 LRU（Least Recently Used）⭐ 默认推荐

**原理**：维护双向链表，每次访问将条目移到链表头部，容量满时淘汰链表尾部（最久未访问）。

**优点**：
- 利用时间局部性原理，近期访问的数据可能再次访问
- O(1) 读写性能
- 自适应访问模式变化

**缺点**：
- 需要额外内存维护链表指针
- 一次性大量扫描会污染缓存

**适用场景**：
- **签名 Nonce 校验**（ncgo 用法）：防重放攻击，TTL 内记录已用 nonce
- **限流计数器**（ncgo 用法）：rate limit buckets/windows
- **幂等性存储**（ncgo 用法）：记录请求处理状态
- 数据库查询结果缓存
- 配置信息缓存
- Session 存储

**samber/hot 用法**：
```go
cache := hot.NewHotCache[string, *rateBucket](hot.LRU, 100000).Build()
cache.SetWithTTL(key, bucket, 5*time.Minute)
```

### 3.3 LFU（Least Frequently Used）

**原理**：维护每个条目的访问频率，容量满时淘汰访问次数最少的条目。通常结合 aging 机制防止历史频率永久影响。

**优点**：
- 保留热点数据，抗扫描污染
- 适合长期稳定的访问模式

**缺点**：
- 需要额外内存维护频率计数
- 新条目可能因频率低被快速淘汰（需 warmup 机制）
- O(log n) 操作复杂度

**适用场景**：
- CDN 缓存（热门资源长期保留）
- 数据库连接池复用统计
- 长期运行的配置缓存
- 访问模式稳定且热点明显

**samber/hot 用法**：
```go
cache := hot.NewHotCache[string, Config](hot.LFU, 500).Build()
```

### 3.4 CLOCK（Clock / Second-Chance）

**原理**：LRU 的近似算法。使用环形缓冲区 + 访问位（reference bit）。指针循环扫描，清除访问位，淘汰未被再次访问的条目。

**优点**：
- 接近 LRU 效果，但无需链表
- 内存开销比 LRU 小
- O(1) 操作

**缺点**：
- 精度不如 LRU（近似算法）
- 需要调优指针扫描参数

**适用场景**：
- 内存受限但需要 LRU 效果
- 操作系统页面置换（经典场景）
- 嵌入式设备缓存
- 大规模缓存（百万级条目）

**samber/hot 用法**：
```go
cache := hot.NewHotCache[string, Page](hot.CLOCK, 1000000).Build()
```

### 3.5 MRU（Most Recently Used）

**原理**：与 LRU 相反，淘汰最近访问的条目。

**优点**：
- 适合特定访问模式
- O(1) 操作

**缺点**：
- 大多数场景效果差
- 使用场景有限

**适用场景**：
- 周期性访问模式（访问后立即不会再次访问）
- 某些特定的文件缓存场景
- 循环播放的媒体资源

**samber/hot 用法**：
```go
cache := hot.NewHotCache[string, MediaChunk](hot.MRU, 100).Build()
```

## 四、go-tools 推荐选择

### 默认推荐：LRU

**理由**：
1. 适用于大多数业务场景
2. ncgo 模板验证（签名/限流/幂等性均使用 LRU）
3. samber/hot 默认策略
4. 时间局部性原理符合常见访问模式

### 场景决策树

```
是否需要保留热点数据？
├─ 是 → 访问频率差异大？
│      ├─ 是 → LFU
│      └─ 否 → LRU（默认）
├─ 否 → 内存是否受限？
│      ├─ 是 → CLOCK
│      └─ 否 → FIFO（最简单）
└─ 访问模式是否周期性？
   └─ 是 → MRU
```

### go-tools 各模块推荐

| 模块 | 推荐算法 | 理由 |
|------|---------|------|
| `go-common/cache` | LRU | 通用场景默认 |
| `go-middleware/redis` | LRU | 连接池、配置缓存 |
| `go-middleware/kafka` | FIFO | 消息暂存缓冲区 |
| `go-framework/hertz` | LRU | 限流、签名校验 |
| `go-framework/kitex` | LRU | 限流、幂等性存储 |

## 五、samber/hot API 速查

### 创建缓存

```go
import "github.com/samber/hot"

// 创建 LRU 缓存，容量 1000
cache := hot.NewHotCache[string, MyValue](hot.LRU, 1000).Build()

// 创建 FIFO 缓存
cache := hot.NewHotCache[string, MyValue](hot.FIFO, 1000).Build()

// 创建 LFU 缓存
cache := hot.NewHotCache[string, MyValue](hot.LFU, 1000).Build()

// 创建 CLOCK 缓存
cache := hot.NewHotCache[string, MyValue](hot.CLOCK, 1000).Build()

// 创建 MRU 缓存
cache := hot.NewHotCache[string, MyValue](hot.MRU, 1000).Build()
```

### 基本操作

```go
// 设置值（无 TTL）
cache.Set(key, value)

// 设置值（带 TTL）
cache.SetWithTTL(key, value, 5*time.Minute)

// 获取值
value, found, err := cache.Get(key)

// 删除
cache.Delete(key)

// 检查是否存在
exists := cache.Has(key)

// 获取缓存大小
size := cache.Len()
```

## 六、迁移计划

### Phase 1 任务 1.3 更新

**原方案**：搬迁 `tools/cache/` → `go-common/cache/`，保留自定义 FIFO/LRU/LFU/CLOCK/MRU 实现

**新方案**：
1. 删除 `tools/cache/core/` 目录（自定义实现）
2. 删除 `github.com/Code-Hex/go-generics-cache` 依赖
3. 添加 `github.com/samber/hot` 依赖
4. 重写 `go-common/cache/` 为 samber/hot 的薄封装
5. 更新 `captcha/` 模块的缓存依赖（如有）

**验收**：
- `cd go-common && go test ./...` 全部通过
- 无 `go-generics-cache` import
- 对齐 ncgo 模板的 `hot.NewHotCache[K,V](policy, size).Build()` 用法
