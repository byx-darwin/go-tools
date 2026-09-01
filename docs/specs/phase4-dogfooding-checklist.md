# Phase 4 Dogfooding Checklist

用于 gf-workflow full 模式 Phase 4：交付前用真实调用路径（而非仅内部单元测试）验证新功能，确认对外 API 可用、行为符合预期。

## 检查项

- [ ] **公共 API 可用**：只用包外可见的导出符号（不借助内部字段/方法）构造并驱动一次完整调用路径
- [ ] **默认行为不变**：未启用新特性时，行为与变更前一致（零破坏性）
- [ ] **新特性按文档生效**：启用新特性后，观察到的实际效果与设计文档/README 描述一致
- [ ] **错误路径可观察**：异常场景下不 panic，错误/告警信息可读、可定位
- [ ] **文档与实际行为一致**：README/godoc 中的示例代码可直接运行，无需额外改写

## 记录格式

针对每次 Phase 4 dogfooding，在下方或独立文件追加一条记录：

```
### <PR 编号> — <一句话描述>

- 验证方式：<如何验证，命令/代码片段>
- 结果：<观察到的现象>
- 结论：✅ 通过 / ⚠️ 有保留 / ❌ 不通过
```

## 记录

### #70 — go-middleware/tls Producer 接入 WithTrace() OTel 追踪

- 验证方式：临时程序仅调用 `tls.NewProducer`/`tls.WithTrace()`/`Producer.SendLog` 三个导出符号（不触碰内部字段），分别验证"未启用追踪"和"启用追踪 + 调用方 ctx 携带 span"两个场景
- 结果：
  - 未启用 `WithTrace()`：`SendLog` 正常返回错误（fake endpoint 网络失败），产生 span 数 = 0，符合"默认行为不变"预期
  - 启用 `WithTrace()`：产生 `tls.flush` span，`status=Error`（PutLogs 网络失败被正确记录）、属性 `tls.topic_id=topic-dogfood`、`tls.batch_size=1`、`links=1` 且 Link 目标即调用方 `handle-request` span——与设计文档"仅 flush 起 span + Link 关联调用方"的方案完全一致
  - 全程无 panic，错误信息可读、可定位（`unsupported protocol scheme`，符合 fake endpoint 预期）
- 结论：✅ 通过

### #84 (Issue #74) — jwt.Refresh Refresh Token 轮换与复用检测

- 验证方式：临时外部测试文件 `package jwt_test`（`go-auth/jwt/dogfood_external_test.go`，运行后删除，未提交），只使用导出符号 `jwt.Sign`/`jwt.Verify`/`jwt.Refresh`/`jwt.ExtractJTI` 与导出接口 `revocation.Store`（自建内存实现），覆盖四个场景
- 结果：
  - 公共 API 端到端可用：`Sign` → `Refresh`（带 `ctx`+自建 `store`）→ `Verify` 全链路通过，`ExtractJTI` 确认新 token 携带全新 JTI（`ff07e1b3-...` ≠ 旧 `jti-dogfood-1`）
  - 默认行为不变：Claims 不带 JTI 时 `Refresh` 成功签发新 token，且 `store.revoked` 始终为空——与"无 JTI 完全跳过轮换"的文档描述一致
  - 新特性按文档生效：用同一旧 token 二次 `Refresh` 返回 `"jti jti-dogfood-2 already used for refresh (reuse detected)"`，与复用检测设计一致
  - 错误路径可观察：`store` 传 `nil` 且 Claims 带 JTI 时，返回清晰错误 `"revocation store is required for tokens carrying jti"`，全程无 panic（`recover()` 断言确认）
  - 文档与实际行为一致：`go-auth/jwt/options.go` 包注释中的新签名用法示例（`ctx`/`store` 参数顺序）与本次实测调用完全一致
- 结论：✅ 通过
