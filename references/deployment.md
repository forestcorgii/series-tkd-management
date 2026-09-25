# Deployment & Build Configuration

### Context: Railway & Nixpacks mise Go Version Resolution

- **Problem**: Railway uses Nixpacks/mise to provision the build environment. mise deprecated relying solely on the minimum `go` directive in `go.mod` (`mise WARN deprecated [idiomatic.go.mod.go-directive]`), requiring explicit toolchain configuration before version 2026.11.0. Adding `toolchain` to `go.mod` can get stripped by `go mod tidy` if the local compiler is newer.
- **Enforced Solution**:
  - Keep a [`.go-version`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/.go-version) file at the project root containing the target version (e.g. `1.22.0`).
  - This informs `mise` / `nixpacks` directly on Railway without conflicting with standard `go mod tidy` operations.
