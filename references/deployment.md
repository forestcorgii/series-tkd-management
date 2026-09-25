# Deployment & Build Configuration

### Context: Railway & Nixpacks mise Go Version Resolution

- **Problem**: Railway uses Nixpacks/mise to provision the build environment. mise deprecated relying solely on the minimum `go` directive in `go.mod` (`mise WARN deprecated [idiomatic.go.mod.go-directive]`), requiring explicit toolchain configuration before version 2026.11.0. Adding `toolchain` to `go.mod` can get stripped by `go mod tidy` if the local compiler is newer.
- **Enforced Solution**:
  - Keep a [`.go-version`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/.go-version) and [`mise.toml`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/mise.toml) (`[tools] go = "1.22.0"`) at the project root.
  - This informs `mise` / `nixpacks` directly without triggering deprecation warnings or conflicting with standard `go mod tidy` operations.

### Context: Nixpacks "no Go files in /app" Build Failure

- **Problem**: Nixpacks defaults the Go build phase command to `go build -ldflags="-w -s" -o out`, executing in the repository root (`/app`). In standard Go project layouts where the entrypoint is located under `cmd/server/main.go`, Nixpacks fails with `no Go files in /app`.
- **Enforced Solution**:
  - Add a [`nixpacks.toml`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/nixpacks.toml) file at the repository root targeting `./cmd/server`:
    ```toml
    [phases.build]
    cmds = ["go build -ldflags=\"-w -s\" -o out ./cmd/server"]

    [start]
    cmd = "./out"
    ```
