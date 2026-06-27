# Test Report

**Generated:** 2026-06-27 08:15:52

## Summary

| Metric | Value |
|--------|-------|
| Total | 39 |
| ✅ Passed | 34 |
| ❌ Failed | 0 |
| ⏭ Skipped | 5 |
| Duration | 1.368s |

## Results by Category

### Health

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | health | 2ms |  |

### go-common

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | common/crypto | 2ms |  |
| ✅ | common/cache | 1ms |  |
| ✅ | common/error | 1ms |  |
| ✅ | common/netutil | 41ms |  |
| ✅ | common/timeutil | 0s |  |
| ✅ | common/log | 1ms |  |
| ✅ | common/httpclient | 1.221s |  |
| ✅ | common/template | 3ms |  |
| ✅ | common/executil | 60ms |  |
| ✅ | common/astutil | 3ms |  |
| ✅ | common/aksk | 0s |  |
| ✅ | common/captcha | 2ms |  |
| ✅ | common/captcha/verify | 4ms |  |

### go-auth

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | auth/jwt/sign | 2ms |  |
| ✅ | auth/jwt/verify | 2ms |  |
| ✅ | auth/jwt/refresh | 1ms |  |
| ✅ | auth/session/create | 3ms |  |
| ✅ | auth/session/get | 0s |  |
| ✅ | auth/device/register | 3ms |  |
| ✅ | auth/device/list | 0s |  |
| ✅ | auth/jwt/sign-device | 1ms |  |
| ✅ | auth/session/delete | 0s |  |
| ✅ | auth/device/remove | 0s |  |

### Auth Protected Routes

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | protected/jwt/no-token | 0s |  |
| ✅ | protected/jwt/valid | 0s |  |
| ✅ | protected/session/valid | 0s |  |
| ✅ | protected/device/valid | 0s |  |

### Middleware

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ⏭ | middleware/redis | 0s | redis not available in local mode |
| ⏭ | middleware/kafka | 0s | kafka not available in local mode |
| ⏭ | middleware/db | 0s | db not available in local mode |
| ⏭ | middleware/es | 0s | elasticsearch not available in local mode |
| ⏭ | middleware/clickhouse | 0s | clickhouse not available in local mode |

### Config

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | config/load | 1ms |  |
| ✅ | config/duration | 1ms |  |
| ✅ | config/hot-reload | 2ms |  |
| ✅ | config/polaris | 1ms |  |

### RPC

| Status | Test | Duration | Details |
|--------|------|----------|--------|
| ✅ | rpc/echo | 9ms |  |
| ✅ | rpc/health | 1ms |  |

