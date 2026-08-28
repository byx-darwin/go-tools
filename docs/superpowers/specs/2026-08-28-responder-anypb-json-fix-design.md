# fix(hertz): Responder JSON 序列化 anypb.Any 展开问题

**Issue:** #52
**Date:** 2026-08-28
**Type:** Bounded（单文件修复）

## 问题

`go-framework/hertz/response.go` 的 `Responder` 在 JSON 响应中使用 `ctx.JSON`（标准 Go JSON 序列化器）序列化包含 `*anypb.Any` 字段的 `Response` 对象，导致前端收到的是 `Any` 的原始格式（`type_url` + `value` base64），而不是展开后的实际消息字段。

## 根因

`reply()` 方法将 protobuf 数据包装为 `*anypb.Any` 放入 `Response.Data` 字段。`writeResponse` 的 JSON 分支使用 `ctx.JSON(httpCode, obj)`，底层是 Go 标准 `encoding/json`，不能正确处理 `anypb.Any` 的 JSON mapping（不会展开为内联消息）。

## 修复方案

修改 `writeResponse` 方法，在 JSON 分支中增加 `proto.Message` 类型断言，优先使用 `protojson.Marshal` 序列化：

```go
func (r *Responder) writeResponse(ctx *app.RequestContext, httpCode int, obj any) {
    switch negotiateContentType(ctx) {
    case consts.MIMEPROTOBUF:
        ctx.ProtoBuf(httpCode, obj)
    default:
        if protoMsg, ok := obj.(proto.Message); ok {
            b, err := protojson.MarshalOptions{
                EmitUnpopulated: false,
                UseProtoNames:   false,
            }.Marshal(protoMsg)
            if err == nil {
                ctx.Data(httpCode, consts.MIMEApplicationJSONUTF8, b)
                return
            }
        }
        ctx.JSON(httpCode, obj)
    }
}
```

## 改动范围

- **文件：** `go-framework/hertz/response.go` — `writeResponse` 方法
- **新增 import：** `google.golang.org/protobuf/encoding/protojson`
- **测试：** `go-framework/hertz/response_test.go` — 新增单元测试验证 proto 消息的 JSON 展开

## 影响评估

| 路径 | 影响 |
|------|------|
| Protobuf 内容协商（Accept: application/protobuf） | ❌ 不受影响，走 `ctx.ProtoBuf` 分支 |
| JSON + proto 消息 | ✅ 修复：展开为业务字段 |
| JSON + 非 proto 对象 | ❌ 不受影响，回退到 `ctx.JSON` |
| `protojson.Marshal` 失败 | ❌ 不受影响，回退到 `ctx.JSON` |
