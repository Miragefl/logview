# PROJECT-CONTEXT.md

# 项目上下文（由 context skill 生成，后续任务直接复用，勿重复询问已有字段）

- language: go
- jdk: 不适用（Go 项目，无 JDK）
- build: go module（github.com/justfun/logview）+ Makefile 封装
- build_cmd: make build（等价 go build -ldflags "..." -o logview .）；纯编译检查用 go build ./...
- run_cmd: go run . <子命令>（cobra CLI，版本号通过 -ldflags 注入 main.version/commit/date）
- test_cmd: go test ./...
- db: 无（依赖中不含任何数据库驱动）
- orm: 无（无 gorm/sqlx/ent 等）
- package: module github.com/justfun/logview；入口 main.go（package main），业务在 ./cmd 与 ./internal
- doc_root: docs/
- modules: cmd（cobra 根命令）；internal/{buffer,export,integration,model,parser,stacktrace,stream,tui}；testutil（测试工具 + 截图）
- notes: 日志查看 TUI 工具（logview）。技术栈：bubbletea/bubbles/lipgloss（TUI）+ cobra（CLI）+ gopkg.in/yaml.v3（配置）。发布用 .goreleaser.yml。测试覆盖见 internal/integration 与 testutil。
- test_build_cmd: go build -ldflags "-X main.version=nightly -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date +%Y-%m-%dT%H:%M:%SZ)" -o ~/Documents/logview-nightly .
- test_build_dir: ~/Documents/ （测试版本固定输出目录，用户本地体验用；正式发布仍走 script/release.sh）
