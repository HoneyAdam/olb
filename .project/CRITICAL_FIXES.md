# Critical Fixes Guide

> Immediate steps to fix production blockers
> Estimated time: 2 hours total

---

## Summary

| Issue | Effort | Impact |
|-------|--------|--------|
| No frontend lockfile | 5 minutes | Blocks security audit |
| CI missing frontend build | 30 minutes | Engine tests fail |
| Frontend security vulnerabilities | 1 hour | Unknown security risks |

---

## Fix 1: Generate Frontend Lockfile (5 minutes)

### The Problem
- No `pnpm-lock.yaml` file in `internal/webui/`
- Cannot run `pnpm audit` without lockfile
- Non-reproducible builds

### The Fix

```bash
# Navigate to frontend directory
cd internal/webui

# Generate lockfile
pnpm install

# Verify it was created
ls -la pnpm-lock.yaml

# Stage for commit
git add pnpm-lock.yaml

# Commit
git commit -m "chore: Add pnpm lockfile for reproducible builds

- Enables security auditing
- Ensures consistent dependencies across environments
- Required for CI/CD"
```

### Verification

```bash
# Check lockfile exists
ls -la pnpm-lock.yaml

# Verify install works from lockfile
rm -rf node_modules
pnpm install --frozen-lockfile
```

---

## Fix 2: Add Frontend Build to CI (30 minutes)

### The Problem
- `//go:embed dist` directive in `internal/webui/embed.go`
- `go test` fails with "pattern dist: no matching files found"
- Engine tests cannot run without built frontend

### The Fix

Edit `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # ===== ADD THIS SECTION =====
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Setup pnpm
        uses: pnpm/action-setup@v2
        with:
          version: 9
          run_install: false

      - name: Get pnpm store directory
        shell: bash
        run: |
          echo "STORE_PATH=$(pnpm store path --silent)" >> $GITHUB_ENV

      - name: Setup pnpm cache
        uses: actions/cache@v4
        with:
          path: ${{ env.STORE_PATH }}
          key: ${{ runner.os }}-pnpm-store-${{ hashFiles('**/pnpm-lock.yaml') }}
          restore-keys: |
            ${{ runner.os }}-pnpm-store-

      - name: Install frontend dependencies
        working-directory: ./internal/webui
        run: pnpm install --frozen-lockfile

      - name: Build frontend
        working-directory: ./internal/webui
        run: pnpm build
      # ===== END OF ADDED SECTION =====

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Run tests
        run: go test -race ./...
```

### Verification

```bash
# Test locally
make test
# or
cd internal/webui && pnpm build
cd ../..
go test ./...
```

---

## Fix 3: Run Frontend Security Audit (1 hour)

### The Problem
- Cannot audit without lockfile (fixed in step 1)
- Need to check for known vulnerabilities
- Need to apply fixes

### The Fix

```bash
# Navigate to frontend
cd internal/webui

# Install dependencies
pnpm install

# Run audit
pnpm audit

# Check if any high/critical vulnerabilities
# If found, run:
pnpm audit --fix

# If automatic fix fails, review manually
pnpm audit --json > audit.json
cat audit.json | jq '.advisories | to_entries[] | select(.value.severity == "high" or .value.severity == "critical")'

# For each critical issue, either:
# 1. Update the package
pnpm update <package-name>

# 2. Or add resolution override in package.json
# {
#   "pnpm": {
#     "overrides": {
#       "vulnerable-package": ">=patched-version"
#     }
#   }
# }

# Re-run audit to verify fixes
pnpm audit
```

### Common Vulnerability Fixes

```bash
# React 19 is very new - check for updates
pnpm update react react-dom

# Tailwind v4 beta - may need to upgrade when stable
# Check for beta updates
pnpm update tailwindcss

# Radix UI packages - update all
pnpm update "@radix-ui/*"

# Axios - common security updates
pnpm update axios
```

### Verification

```bash
# Should show no high/critical vulnerabilities
pnpm audit --audit-level high

# Output should be:
# No known vulnerabilities found
```

---

## Complete Fix Script

Save as `fix-critical-issues.sh`:

```bash
#!/bin/bash
set -e

echo "=========================================="
echo "OpenLoadBalancer Critical Fixes"
echo "=========================================="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

print_info() {
    echo -e "${YELLOW}[i]${NC} $1"
}

# Check prerequisites
print_info "Checking prerequisites..."

if ! command -v pnpm &> /dev/null; then
    print_error "pnpm is not installed. Install with: npm install -g pnpm"
    exit 1
fi

if ! command -v node &> /dev/null; then
    print_error "Node.js is not installed"
    exit 1
fi

print_status "Prerequisites OK"

# Fix 1: Generate lockfile
print_info "Fix 1: Generating frontend lockfile..."
cd internal/webui

if [ -f "pnpm-lock.yaml" ]; then
    print_status "Lockfile already exists"
else
    pnpm install
    print_status "Lockfile generated"
fi

# Stage for commit
git add pnpm-lock.yaml 2>/dev/null || true
print_status "Lockfile staged"

cd ../..

# Fix 2: Build frontend
print_info "Fix 2: Building frontend..."
cd internal/webui

if [ ! -d "dist" ]; then
    pnpm build
    print_status "Frontend built"
else
    print_status "Frontend already built"
fi

cd ../..

# Fix 3: Security audit
print_info "Fix 3: Running security audit..."
cd internal/webui

# Check for vulnerabilities
VULNS=$(pnpm audit --json 2>/dev/null | jq -r '.metadata.vulnerabilities.high + .metadata.vulnerabilities.critical' || echo "0")

if [ "$VULNS" -gt 0 ]; then
    print_info "Found $VULNS high/critical vulnerabilities, attempting to fix..."
    pnpm audit --fix || true
    
    # Check again
    VULNS=$(pnpm audit --json 2>/dev/null | jq -r '.metadata.vulnerabilities.high + .metadata.vulnerabilities.critical' || echo "0")
    
    if [ "$VULNS" -gt 0 ]; then
        print_error "$VULNS vulnerabilities remain, manual review needed"
        pnpm audit
    else
        print_status "All high/critical vulnerabilities fixed"
    fi
else
    print_status "No high/critical vulnerabilities found"
fi

cd ../..

# Verify Go tests work
print_info "Verifying Go tests..."
if go test ./internal/engine/... -run TestEngine 2>&1 | grep -q "pattern dist: no matching files"; then
    print_error "Go tests still failing - dist/ directory not found"
    print_info "Make sure frontend is built: cd internal/webui && pnpm build"
    exit 1
else
    print_status "Go tests can access embedded frontend"
fi

# Summary
echo ""
echo "=========================================="
echo "Fixes Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "1. Review changes: git diff --staged"
echo "2. Commit: git commit -m 'fix: Critical production blockers'"
echo "3. Push: git push"
echo "4. Verify CI passes on the PR"
echo ""
```

Make executable and run:
```bash
chmod +x fix-critical-issues.sh
./fix-critical-issues.sh
```

---

## Bonus Fixes

### Add Dependabot (10 minutes)

Create `.github/dependabot.yml`:

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: "/"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 10
    
  - package-ecosystem: npm
    directory: "/internal/webui"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 10
    
  - package-ecosystem: github-actions
    directory: "/"
    schedule:
      interval: "weekly"
```

### Make Race Detector Blocking (5 minutes)

Edit `.github/workflows/ci.yml`:

```yaml
      - name: Run tests
        run: go test -race ./...
        # This already fails on race, but ensure it's not allowed to fail:
        continue-on-error: false  # Add this line
```

---

## Verification Checklist

After all fixes:

- [ ] `pnpm-lock.yaml` exists in `internal/webui/`
- [ ] `pnpm audit` shows no high/critical vulnerabilities
- [ ] `cd internal/webui && pnpm build` succeeds
- [ ] `go test ./...` passes without "pattern dist" error
- [ ] CI workflow includes Node.js setup
- [ ] CI workflow includes pnpm install
- [ ] CI workflow includes pnpm build before Go tests
- [ ] Changes are committed
- [ ] CI passes on the commit

---

## Expected Results

After fixes:

| Metric | Before | After |
|--------|--------|-------|
| Frontend lockfile | ❌ Missing | ✅ Present |
| Security audit | ❌ Blocked | ✅ Clean |
| Engine tests | ❌ Failing | ✅ Passing |
| CI status | ❌ Red | ✅ Green |
| Overall score | 82/100 | 88-90/100 |

---

*Fixes should be applied in order and committed together*
