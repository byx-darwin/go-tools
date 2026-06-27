# Test Report

**Generated:** 2026-06-27 08:24:19

## Summary

| Metric | Value |
|--------|-------|
| Total | 39 |
| ✅ Passed | 34 |
| ❌ Failed | 0 |
| ⏭ Skipped | 5 |
| Duration | 1.424s |

## Results by Category

### Health

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | health | 2.527ms |  |

### go-common

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | common/crypto | 207µs |  |
| ✅ | common/cache | 121µs |  |
| ✅ | common/error | 167µs |  |
| ✅ | common/netutil | 41.798ms |  |
| ✅ | common/timeutil | 575µs |  |
| ✅ | common/log | 765µs |  |
| ✅ | common/httpclient | 1.314631s |  |
| ✅ | common/template | 1.213ms |  |
| ✅ | common/executil | 50.159ms |  |
| ✅ | common/astutil | 664µs |  |
| ✅ | common/aksk | 310µs |  |
| ✅ | common/captcha | 1.802ms |  |
| ✅ | common/captcha/verify | 1.158ms |  |

### go-auth

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | auth/jwt/sign | 301µs |  |
| ✅ | auth/jwt/verify | 402µs |  |
| ✅ | auth/jwt/refresh | 203µs |  |
| ✅ | auth/session/create | 214µs |  |
| ✅ | auth/session/get | 147µs |  |
| ✅ | auth/device/register | 181µs |  |
| ✅ | auth/device/list | 110µs |  |
| ✅ | auth/jwt/sign-device | 172µs |  |
| ✅ | auth/session/delete | 129µs |  |
| ✅ | auth/device/remove | 123µs |  |

### Auth Protected Routes

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | protected/jwt/no-token | 126µs |  |
| ✅ | protected/jwt/valid | 131µs |  |
| ✅ | protected/session/valid | 129µs |  |
| ✅ | protected/device/valid | 150µs |  |

### Middleware

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ⏭ | middleware/redis | 0µs | redis not available in local mode |
| ⏭ | middleware/kafka | 0µs | kafka not available in local mode |
| ⏭ | middleware/db | 0µs | db not available in local mode |
| ⏭ | middleware/es | 0µs | elasticsearch not available in local mode |
| ⏭ | middleware/clickhouse | 0µs | clickhouse not available in local mode |

### Config

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | config/load | 950µs |  |
| ✅ | config/duration | 645µs |  |
| ✅ | config/hot-reload | 1.391ms |  |
| ✅ | config/polaris | 583µs |  |

### RPC

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | rpc/echo | 1.026ms |  |
| ✅ | rpc/health | 285µs |  |

