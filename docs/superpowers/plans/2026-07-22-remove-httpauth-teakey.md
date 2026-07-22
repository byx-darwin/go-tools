# Remove HTTPAuth.TeaKey Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the orphaned `HTTPAuth.TeaKey` config field and all its references, eliminating a misleading dead field left over from the TEA→AES-GCM migration (#25).

**Architecture:** Pure deletion across 4 files — remove the struct field, update tests, clean the sensitive-key mask list, and remove the example YAML entry. No new code is introduced. `yaml.Unmarshal` ignores unknown fields, so existing deployed configs with `tea_key:` remain backward-compatible.

**Tech Stack:** Go 1.25+, gopkg.in/yaml.v3, testify

## Global Constraints

- Config structs are ncgo scaffold source-of-truth — this is a source-breaking change for any Go code referencing `HTTPAuth.TeaKey` (confirmed: none exists in this repo)
- `yaml.Unmarshal` (non-strict) silently ignores unknown fields — no runtime breakage for existing YAML configs
- All exported symbols require godoc comments (revive linter)
- gofmt-clean, golangci-lint v2 passing
- ncgo template update is out of scope (separate repo, follow-up issue)

---

### Task 1: Remove TeaKey from HTTPAuth struct and update tests

**Files:**
- Modify: `go-framework/config/hertz/config.go:31-36`
- Modify: `go-framework/config/hertz/config_test.go:28-44`

**Interfaces:**
- Produces: `HTTPAuth` struct with only `Enable`, `AK`, `SK` fields

- [ ] **Step 1: Update the test to remove TeaKey references**

In `go-framework/config/hertz/config_test.go`, remove `TeaKey` from the struct literal and its assertion:

```go
func TestServerConfig_Full(t *testing.T) {
	c := &ServerConfig{
		HTTP: &HTTPOption{
			Network:      "tcp",
			Port:         "8080",
			Mode:         0,
			ExitWaitTime: 5 * time.Second,
			IdleTimeout:  30 * time.Second,
			IsTransport:  true,
			IsCors:       true,
			IsRecovery:   true,
		},
		Auth: &HTTPAuth{
			Enable: true,
			AK:     "test-ak",
			SK:     "test-sk",
		},
	}

	assert.Equal(t, "tcp", c.HTTP.Network)
	assert.Equal(t, "8080", c.HTTP.Port)
	assert.Equal(t, 5*time.Second, c.HTTP.ExitWaitTime)
	assert.Equal(t, 30*time.Second, c.HTTP.IdleTimeout)
	assert.True(t, c.HTTP.IsCors)
	assert.True(t, c.HTTP.IsRecovery)
	assert.True(t, c.Auth.Enable)
	assert.Equal(t, "test-ak", c.Auth.AK)
}
```

- [ ] **Step 2: Run test to verify it fails (field still exists but unused)**

Run: `cd /Volumes/SSD/workspace/github.com/byx-darwin/go-tools && go test ./go-framework/config/hertz/... -run TestServerConfig_Full -count=1 -v`

Expected: PASS (the struct still has TeaKey, but test no longer sets it — this confirms the test compiles without referencing TeaKey)

- [ ] **Step 3: Remove TeaKey field from HTTPAuth struct**

In `go-framework/config/hertz/config.go`, delete line 35:

```go
// HTTPAuth 鉴权配置
type HTTPAuth struct {
	Enable bool   `json:"enable"  yaml:"enable"`
	AK     string `json:"ak"  yaml:"ak"`
	SK     string `json:"sk"  yaml:"sk"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Volumes/SSD/workspace/github.com/byx-darwin/go-tools && go test ./go-framework/config/hertz/... -count=1 -v`

Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add go-framework/config/hertz/config.go go-framework/config/hertz/config_test.go
git commit -m "refactor(go-framework): remove orphaned HTTPAuth.TeaKey field

TeaKey was left over from the TEA encryption removal (#25, PR #45).
The AK/SK auth middleware uses pure HMAC-SHA256 and never reads this field.
yaml.Unmarshal ignores unknown fields so existing configs are unaffected.

Closes #46"
```

---

### Task 2: Clean up example references

**Files:**
- Modify: `example/handler/config_handler.go:204`
- Modify: `example/config.yaml:71`

**Interfaces:**
- Consumes: Task 1 completed (TeaKey field removed)
- Produces: No remaining `tea_key` references in example code

- [ ] **Step 1: Remove "tea_key" from sensitive field mask list**

In `example/handler/config_handler.go:204`, change:

```go
for _, s := range []string{"secret", "password", "token", "sk", "tea_key", "app_key"} {
```

to:

```go
for _, s := range []string{"secret", "password", "token", "sk", "app_key"} {
```

- [ ] **Step 2: Remove tea_key line from example config**

In `example/config.yaml`, delete line 71 (`tea_key: "0123456789abcdef"`). The auth section becomes:

```yaml
  auth:
    enable: true
    ak: "example-ak"
    sk: "example-sk"
```

- [ ] **Step 3: Verify no remaining tea_key references**

Run: `cd /Volumes/SSD/workspace/github.com/byx-darwin/go-tools && grep -rn "TeaKey\|tea_key" --include="*.go" --include="*.yaml" --include="*.yml" .`

Expected: No output (zero matches in Go/YAML files; docs/specs references are acceptable)

- [ ] **Step 4: Commit**

```bash
git add example/handler/config_handler.go example/config.yaml
git commit -m "chore(example): remove tea_key from sensitive mask list and example config"
```

---

### Task 3: Full validation

**Files:** None (validation only)

- [ ] **Step 1: Build all modules**

Run: `cd /Volumes/SSD/workspace/github.com/byx-darwin/go-tools && go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`

Expected: No errors

- [ ] **Step 2: Vet**

Run: `cd /Volumes/SSD/workspace/github.com/byx-darwin/go-tools && go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`

Expected: No issues

- [ ] **Step 3: Lint go-framework module**

Run: `cd /Volumes/SSD/workspace/github.com/byx-darwin/go-tools && golangci-lint run --timeout=5m ./go-framework/...`

Expected: No issues

- [ ] **Step 4: Run go-framework tests**

Run: `cd /Volumes/SSD/workspace/github.com/byx-darwin/go-tools && go test ./go-framework/... -count=1`

Expected: ALL PASS

- [ ] **Step 5: Run full workspace tests**

Run: `cd /Volumes/SSD/workspace/github.com/byx-darwin/go-tools && go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`

Expected: ALL PASS
