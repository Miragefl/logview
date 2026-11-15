# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

LogView 是 Go 编写的终端日志查看器(TUI):支持本地文件 / 管道 / SSH / K8s / FRP 隧道多日志源,自动解析 java-logback / JSON / 纯文本格式。文档与注释使用中文。

## 知识库归属

- 项目:logview(`~/Obsidian/10-Projects/logview/`,个人开源项目,无 DB/接口交付物豁免)
- 分支映射:main → grep-context(grep 上下文混入+管道 DSL,已完成,v0.17.4 发布,含 2026-09-14~15 验收迭代:sticky 多锚混入/主界面 j/k;2026-10-09 修正连按合并输入被吞,v0.18.1 发布);main → upgrade-exdev-fix(自升级跨设备 EXDEV 修复:rename 失败兜底同目录 copy+rename,2026-10-10);main → keybind-unify(导航键统一 C-j/C-k,已完成,v0.17.2 发布,主日志界面已于 v0.17.4 改回 j/k,弹窗类界面仍 C-j/C-k);main → search-input-perf(搜索输入性能优化,已完成,v0.17.2 发布);main → gzip-log-support(gzip 日志支持,已完成,v0.17.0 发布);main → keyword-history-freq(关键词历史频次+拆词统计+源 scope 隔离,已完成,v0.17.0 发布,含 2026-09-08 历史追加修正 v0.17.1 发布);main → source-picker-env(源选择器环境过滤,已完成,v0.16.0 发布);main → time-range-search(时间范围搜索 time: 语法,已完成,v0.16.0 发布);main → tui-overhaul(TUI 界面翻新,已完成);main → search-hl-split(搜索命中与高亮词视觉区分:高亮词=10 色亮色系粗体下划线;2026-10-12 迭代:搜索命中弃黄底块改暗色系 8 色轮换粗体,明暗双域区分,v0.19.0 发布);main → perf-1g-load(1G 大文件加载性能迭代,v0.18.0 发布九迭代:列存模型+通道批量化+尾读/--all+分帧搜索刷新+状态栏内存/加载徽章,1G/664 万行加载 300s→28s、--all RSS 1.8G→2.3G、默认档 71MB;2026-10-11 迭代 10 内存再压缩:默认档 GOGC 50+FreeOSMemory+徽章 Sys-HeapReleased 口径+base 去重驻留 --all 活对象再 -106MB;2026-10-12 迭代 11 mmap 零拷贝:≥64MB 文件全量读原文出堆直指映射区,5G/3325 万行 --all 真实占用(phys_footprint)5.0G vs 改前 ~11G 独占;迭代 12 搜索分片 8 worker 并行(664 万行全扫 25s+→≤3s,打字不卡);迭代 13 加载解析并行化(1G 28s→15s、5G 176s→74s 完全线性,真凶系正则串行非渲染);另有混入视口跟锚修复(锚定后 zz 居中一次+连续命中段提示),v0.19.0 发布)。历史需求(initial-build/multi-k8s-resource/search-syntax-enhancement/search-history-popup/frp-stcp-source/source-picker-v2/code-refactor/builtin-themes)已归档 vault,开工前先 ls 需求目录; feature/fview-desktop → main(fview 桌面版 Wails MVP:M1-M3 已完成——logcore 抽取/合仓/发布双产物/tab 会话/虚拟滚动/follow/gzip/搜索/过滤/共享 rules.yaml/主题,Final Review 修复 pass 后 13 包+race 全绿;M4-M7 连接管家/FRP/k8s/打磨待续;详见 vault 10-Projects/fview); feature/fview-ui-vue → fview(fview 桌面版 UI 验收迭代,续用 10-Projects/fview:2026-10-30 第二轮 8 项——k8s/FRP 面板弹窗交互 bug/TUI 式分栏/搜索+高亮+隐藏统一弹窗/vim 键全量回归/主题日志字段配色/顶栏文件名+源信息/设置图标统一/字体下拉宽度);main → titlebar-merge(mac 标题栏融合 tab 栏 IDEA 式:HiddenInset+78px 红绿灯让位+drag-spacer,2026-11-03 spike 验收通过;含同日字号放大布局修复——写死 px 容器全改 --ui-font-size 派生,及关于页版本注入/检测更新 gitee 回落);main → edit-menu-paste(mac ⌘V 粘贴修复:自定义应用菜单替换 Wails 默认菜单后无 Edit 菜单,WKWebView paste: 无人认领致 FRP 表单等所有输入框粘不了,menu.go 补 menu.EditMenu() 官方 Role,2026-11-04,续用 10-Projects/fview;含同日 TUI FRP 新建表单 q 误退修复:q-退出只认输入框空、表单字段每步从空起,首字母 q 直接杀 TUI,sourcepicker_ui q-退出排除表单层 pickerFRPLevel==1);main → panel-search-unify(源面板搜索+新增统一 r1+r2:r1 PanelSearch 常驻框/⌃a 新增统一/SSH 弹窗/⌃c⌃n+Final Review 两修;r2 真机验收改无框打字过滤+高亮+Esc 逐级(panelFilter 会话级共享)/d/e→⌃d⌃e/HelpModal 分组双列,302→309 测试绿+Final Review 四修(I1 模板残迹>/q? 元键放行/⌃c⌃n 无条件消费)+子序列匹配(hde→hardware)+K8s 多资源(Space 多选/Enter 合流 ⌘Enter 各开/来源列,MultiK8sSource 存量接线+Final Review 六修(chip 定宽/hasSource 来源不唯一判定/合流失败提示/动作清标记等),315 测试绿),2026-11-04,续用 10-Projects/fview);main → vim-copy(字符级光标+vim 移动+可视选择复制双端:h/l/w/b/e/0/$/^ 字符光标、v 字符选择+V 行级、yy 整行/visual 内 y 选区、fview 拖选字符级松手即复制;键位腾挪:高亮→ctrl+h、折叠→ctrl+e、换行→ctrl+w(fview ctrl+w 弃关 tab,mac ⌘W 原生菜单保留,Win/Linux 关 tab 只剩 tab 栏 ×);TUI 不做鼠标保持终端原生;2026-11-05 r1 十任务+Final Review(opus)五修完成(临时 wrap 死代码根治/直入 v 锚列/选区行强制 wrap/窗外松手终点/visual 连按 y),双端测试全绿 fview 369+Go 15 包,charMotion/charnav 同用例表对齐,fview.app md5 e766a3da+logview md5 93b56e11 待真机验收;2026-11-05 r2 真机验收三项:隐藏键 x→ctrl+x 双端同步(顺手修 ctrl+e 孤儿弹窗残留)/WordsPopup 删词改 ctrl+d(根因=自动聚焦 input+!inInput 防御真机恒失效,测试 dispatch target 恒 document 伪造不出,补 keyOnInput 测试)/k8s 面板 context/ns NSelect tiny→small,fview 371+Go 全绿,fview.app md5 ba341c34+logview md5 b9b594d2 真机验收通过,v0.22.0 发布,独立需求目录 10-Projects/fview/vim-copy;2026-11-08 r3 三小修:WordsPopup 底部提示文案 d→ctrl+d(r2 只改按键漏同步文案,用户照文案按 d 恒失效)+表头列宽拖拽把手常驻 1px 竖线标记(原常态透明找不到把手)+字段下拉补 ⌃j⌃k 键盘流(NDropdown naive 无 ctrl 导航且全局 ctrl+j/k=动日志光标,capture 中继为方向键+弹窗栈让位),fview 372 绿+md5 1b35afae 交付;r5 真机修复:合成方向键改派发 activeElement+bubbles——evtd 委托挂 window 按 target→window 链匹配,r4 派发 document 本体匹配不到(测试 spy 挂 document 层假绿,对照组原生 ArrowDown 实证),测试升 attachTo body+spy 挂 window+pending class 端到端,md5 1192f7b0 交付;r6:naive 键盘表无 Space→拦 Space 合成 Enter 勾选,Esc 因 manual 受控未接 update:show→拦 Esc 自关(故意不接回调,保住 Enter 连续勾选不关),onFieldMenuKey 三键统一,373 绿+md5 ac445b81 交付);main → builtin-themes-r2(内置主题第二轮 +9 删 5 终态 17 款:增 solarized-dark/kanagawa/everforest/rose-pine-moon/monokai-pro/synthwave-84/one-half-light/rose-pine-dawn/ayu-light,删 nord/catppuccin/one-dark/catppuccin-latte/nord-light,被删名回落 dark;浏览器样张验收选型,默认模板名单一致性测试堵双源漂移,v0.22.2 发布,续用 10-Projects/logview/builtin-themes);main → github-seo(GitHub 功能词可发现性:description 重写+README 双语首段/Use Cases+topics 补 5;module path justfun→Miragefl 修正,go install 自 v0.22.2 可用;"kubectl logs alternative" 搜索第 1,10-Projects/logview/github-seo);main → readme-slim(README 折叠化瘦身:安装/快捷键/rules.yaml 模板/命令补全收 details,可见 450→256 行,SEO 段零动,v0.22.2 发布,10-Projects/logview/readme-slim);main → field-interact(字段列交互三件套,仅 fview 桌面版:双击字段复制原始值/光标词+visual 选区一键带入搜索高亮隐藏输入框/h 跳字段域+yy 复制栏位,10-Projects/fview/field-interact);main → field-match-scope(搜索/高亮/隐藏匹配域统一全行:裸词翻案改 raw 全行+message: 兜底、高亮/命中渲染扩字段列,logcore 双端同变,10-Projects/fview/field-match-scope);main → vim-count(vim 数字前缀+tab 数字角标+Q 反向循环面板,双端:count 生效键 j/k/h/l/w/b/e/G/g/⌃d/u/f/b,0 无 count=行首/有 count 并入,clamp 1000,Esc/非移动键清 count;fview keymap pendingCount 状态机+StatusBar showcmd 反馈+TUI countN 含连按合并拆解("10j"单条 KeyRunes);tab 栏 ≤9 显示 dim 角标提示 ⌘N;Q 反向循环源面板(仅 fview,TUI q=开源选择器无对应);Final Review 后 fview 442 测试绿+go 全绿,fview.app md5 74bfe2ce+logview-nightly md5 71c1a759(Final Review 两修:TUI count+G autoscroll 钉尾致跳行无效改 n<=0+渲染级测试锁定/TUI h/l 补 count 对齐双端)交付待真机验收,2026-11-12,10-Projects/fview/vim-count);main → panel-hot-sort(SSH/FRP/K8s 三源面板热点排序:默认热度序(衰减分降序+同分名称序)+★≥0.99+过滤只筛不排;Go UsageScores 批量分接口+Open* 补 Bump(ssh:/frp: 双端互通,k8sres 保持 fview 四段 ctx/ns/kind/name 因 recent.ts 直开拆解依赖全参,Ruling 修订不与 TUI 两段互通,k8sctx/k8sns 用户下拉选择时记);fview 451 测试绿+go 全绿,fview.app md5 0852a645 交付待真机验收,2026-11-12,10-Projects/fview/panel-hot-sort);main → tui-word-field(TUI 补齐 fview 两能力:①光标词直达——wordAt/cursorWord Go 版(标点空白返回空,fview 验收#6 同款)三级优先 visual 选区词/字段域整字段值/message 光标词,搜索/高亮/隐藏三入口预填(光标词>已有词 join>空,非空不覆盖)②字段域导航——fieldFocus 列域光标(col0 再 h 跳最右栏/h/l 两端停/最右 l 回 message/motion 回跳/跨行保持逐行扫列/yy 复制栏位原始值/双渲染路径逆视频高亮);Ruling:fieldFocus 跨行保持(对齐 fview,推翻计划切行复位)、命名避撞 F 面板 fieldCursor;go 全绿+Final Review 四修(beginVisual 清字段域防 visual h/l 劫持/合并 yy 直达分派栏位/visual 下 //C-h/C-x 透传弹窗使选区词预填可达/空栏位 yy 报未复制)后 logview-nightly md5 a6016b7a 交付待真机验收,2026-11-13,10-Projects/fview/tui-word-field);main → frp-pwd-focus(fview BrowseDialog 密码框聚焦修复:FRP/SSH 认证失败 needPwd 弹密码框但 NModal auto-focus=false 且无聚焦代码,焦点留 body——打字被 onDocKey typeKey 吃进过滤词、Enter 被让位逻辑吞掉,键盘重输链断即"登录失败不能重新输入密码";修=watch(needPwd) nextTick 聚焦(WordsPopup 先例同款),根因定位走 systematic-debugging 对照工作样例;browsedialog 22/22+全量 453/453+mutation 验证,fview.app md5 b7034e5d;2026-11-13 TUI 侧同修:promptFRPPw 的 user=="" 守卫吞密码框(user 空合法配置默认 root,错误上屏无重输口)+487 路径直达裸 user@host 拼接改 FRPDefaultUser 兜底,TestFRPPwdPrompt RED→GREEN,logview-nightly md5 27e591d6,交付待真机验收,10-Projects/fview/frp-pwd-focus);main → tui-picker-unify(双端键位对齐三工程之一·TUI picker 向 fview 面板键系靠拢:①无框打字过滤全层化——ctx/资源/本地/SSH 目录+主机过滤态/FRP L0 挂 pickerDirFilter(切层/tab/下钻清词),ns 层保持输入直达,SSH ⌃a 输入态与 FRP 表单为输入语义不无框化②isSubseq/splitHighlight Go 版从 fview highlight.ts 逐用例移植(子串优先+子序列退化,同表测试)候选行金粗体高亮③Esc 逐级(词→层→关)④⌃d 删 FRP(C-x 退役)/⌃e 编辑回填表单/⌃a 新增统一⑤K8s ⌃c⌃n 直达(Ruling:ctrl+c picker 态放行,picker 外退出惯例不变);顺手清账 vim-count deferred#3(pgup/pgdn 乘 count)+C 清屏补列;Ruling:frpConnCandidates 词态 +new 泄漏修复;存量断言迁移 3 处;go 全绿+Final Review 六修(Esc 逐级改层级回退专用路径 pickerLevelBack·Ruling 修正 fview 准绳"Esc 不逐目录上翻"/⌃a 表单守卫+隧道清理/ns 层词清统一 inputRef/SSH·FRP 目录层补 isSubseq/三无框层 ⌕ 回显+死框退役/q 守卫补 SSH 输入态,另 M7/M8/M11 顺手)+logview-nightly md5 499964f4 交付待真机验收,2026-11-13,10-Projects/fview/tui-picker-unify);main → frp-alias(TUI FRP 连接别名:表单末步加可空 alias(stcp step6/tcp step5)+⌃e 编辑回填带原值+submitFRPForm 组 Conn 带 Alias 修编辑静默抹别名 bug+主界面 label frp://别名空回落 name 对齐 fview tab 语义;顺手根治 +new Enter 内联不清编辑残值缺陷改调 pickerNewFRPForm;Alias 纯展示不进逻辑键,L0 过滤 label 已含 alias 零改动,fview 桌面端零改动;go 全绿+新增 4 测试+存量适配 4+mutation 验证接线+Final Review(fable)零 Critical/Important 两 Minor 修毕(app.go 步骤注释/stcp 接线端到端锁),logview-nightly md5 d5c32342 交付待真机验收,2026-11-13,10-Projects/fview/frp-alias);main → frp-edit-prefill(TUI FRP 编辑表单逐步预填:frpSeedStepInput 进入每步带出当前值+光标置 runes 尾(Enter 保留改哪动哪),编辑 addr 种子含 :port 端口完整往返(修非默认端口编辑一轮被吞 7000),User 空显式化 root 解必填卡死,type 光标落原类型行,回退再前进不丢已输值(新表单同修),submitFRPForm 按型清空对侧字段防编辑切型残留(对齐 fview isTCP);⌃u 清空系 textinput.go 既有能力零新增;Final Review(fable)零 Critical 一 Important 修毕(⌃a/⌃e 无编辑/新增语义的层改转发输入编辑行首行尾——预填光标常态在尾,吞键缺陷暴露面被放大);存量 TestFRPEditKeepsAlias/ClearsAlias 按预填语义改写(编辑全程 7 连 Enter 零打字直通);go 15 包全绿+logview-nightly md5 5e57678c1ccdf3d40d0aed2d879cd0d9 交付待真机验收,2026-11-14,10-Projects/fview/frp-edit-prefill);main → picker-width-hints(TUI picker 弹窗动态宽度+底部快捷键按需沉底:①pickerBoxW=clamp(64,内容最长行+2,width-4) 内容驱动,buildSourcePickerLines 两阶段构建(tab 行/tabSep 延后随 boxW 渲染),超宽 FRP 连接行不再 wrap、短内容 64 视觉零变化、窄终端封顶,顺修存量"（未选过日志）"全角括号 1 字符 wrap;②pickerShortcutItems 十状态键位表(正本 handleSourcePickerKeys 实测)替换 sourcePickerMode 老静态分支——K8s ctx/ns/资源(Space 多选仅资源层)/本地/SSH 主机过滤态/输入态(⌃l)/目录层/FRP L0(⌃a⌃e⌃d)/表单/L2 各按需显示;③弹窗内快捷键 hint 删净(SSH/FRP L0 两处,"打字过滤·⌃a 新增主机"等),⌕ 回显/空态文案保留,顺手修"（C-k 切到路径输入）"过时文案(切焦点实为 ⌃l);TDD 全程 RED→GREEN+Final Review(fable)零 Critical 两 Important 修毕(SSH 输入态"Enter 直连"系假承诺——pickerEnter 越界分支只路由 K8s ns/FRP 表单,SSH 输入态穿透 return nil,HEAD 即死待另行立项,文案改口"无匹配主机";⌃l 切焦点切到无渲染路径的隐形路径框,底部栏去该项键位保留)+go 15 包全绿+logview-nightly md5 c18532cfa18f87929a992fc480e88948 交付待真机验收,2026-11-14,10-Projects/logview/picker-width-hints);main → frp-path-hide(TUI FRP 选择器 L0 行不显示日志路径:纯显示层去路径降噪+删"（未选过日志）"占位,过滤域随 label 所见即所筛;数据链路/⌃e 回填/建连零影响;go 15 包全绿+logview-nightly md5 fd1cbe64 交付待真机验收,2026-11-15,10-Projects/logview/frp-path-hide);main → keybind-unify-r2(双端快捷键完全统一,fview 为准,2026-11-15 完成待真机验收:审计四分类落 specs+拍板 A1 q 关 picker 永不杀程序/A2 标记列表浮层(' 开/⌃j⌃k/Enter 居中即关,Esc 链=详情→统计→列表→清词)/A3 SSH ⌃a 保存型表单+⌃d 删主机(cfg 源跳过,SetSSHHosts 双源注入)/A4 K8s ⌃c⌃n 豁免;B1 目录层 ⌃h 回上级/B2 搜索弹窗词表+⌃d 删词(历史态不删)/fview 补 ↑↓←→+preventDefault 白名单/A8 帮助任意键关;双端 help 清漂移(TUI 9 条含 D 档位旧文案,fview 7 条含 ctrl+w 关 tab 假条目根除);Final Review(fable)0 Critical 5 Important 全修(历史态⌃d误删/删词Enter复活/目录层⌃a隐形表单/跳转居中/写回吞错)+3 Declined Ruling+9 Minor 延后;go 15 包+vitest 457 全绿,logview-nightly md5 82989683+fview.app md5 175f7eca 交付待真机验收;2026-11-15 验收迭代 r1:快捷键显示 ⌃/ctrl+ 全部统一 C- 缩写(键名匹配逻辑与 ⌘ 系零触碰,⌃ 不吃 keycapHint 键帽样式顺带修复),nightly md5 25720074+fview md5 3dbffc30 交付待真机验收,10-Projects/fview/keybind-unify-r2)

## 常用命令

```bash
make build              # 构建单文件二进制 logview(ldflags 注入 main.version/commit/date)
go build ./...          # 纯编译检查
go test ./...           # 全量测试(~187 个 Test 函数,几十秒)
go test ./internal/tui/ -run TestXxx -v   # 跑单个测试(标准 go test,无外部断言库)

# FRP 集成测试(build tag 隔离,需真实 frpc + 可达 frps + FRPS_ADDR/FRPS_TOKEN/FRP_SK/FRP_PROXY 环境变量)
go test -tags frpintegration ./internal/frp/ -run TestStartTunnelReal -v
```

- 无 lint 配置(无 .golangci.yml),无测试类 CI;`.github/workflows/` 仅有 Gitee 镜像同步。
- 发布走 `script/release.sh [patch|minor|major]`(goreleaser + brew tap);日常体验版构建见 PROJECT-CONTEXT.md 的 test_build_cmd。
- fview 桌面版本地验收包:`script/build-fview.sh`(git describe 推版本注入 ldflags——裸 `wails build` 的 version 是空串,关于页只显示 dev;脚本自动删旧包重拷 `~/Documents/` 并 md5 双端核对;`--keep` 只构建不交付)。

## 架构

### 数据流

```
LogStream.Start() → chan model.RawLine → tui.App.Update(batchMsg)
  → parser.AutoDetect 探测 → Parser.Parse → buffer.RingBuffer(容量 = --buffer-size)
  → 过滤/搜索 → logview.go 渲染 → View()
```

### 入口与装配

- `main.go` 只声明版本变量 + 委托 `cmd.Execute()`;**全部 CLI 在 `cmd/root.go` 单文件**:子命令 `picker / k8s / tail / file / pipe / upgrade / completion`,裸 `logview` 在 `Execute()` 里自动补子命令(stdin 是管道 → pipe,TTY → picker)。
- `expandTailArgs` 支持 `-200f` 语法糖(展开为 `--tail 200 -f`)。
- `runTUI()`(root.go)是统一装配点:`tui.NewApp` + AltScreen,退出时 `frp.KillAllTunnels()` 兜底。
- Persistent flags:`--rule`(指定解析规则,空则自动探测)、`--buffer-size`(默认 100000)、`--config`(配置目录,默认 `~/.config/logview`)。

### internal/ 各包职责

| 包 | 职责 |
|---|---|
| `stream/` | `LogStream` 接口(`Start/Label/Cleanup`,stream.go)+ 各日志源。K8s 与 SSH 均 **exec 系统命令**(`kubectl`/`ssh`),不用 SDK;`sshconfig.go` 解析 `~/.ssh/config` |
| `parser/` | `Parser` 接口 + `RegexParser`(命名捕获组)/ `JSONParser`;`rules.go` 加载 rules.yaml、`AutoDetect` 按源逐条探测(pending 每源最多 50 行,超限回落 plain-text,后续命中可升级解析器并回灌重解析) |
| `model/` | `RawLine{Text, Source, Seq}` → `ParsedLine`(7 个标准 Field,`FieldMask` 控制显隐,`AllFields` 可被配置覆盖) |
| `buffer/` | 定容环形缓冲存 `*ParsedLine`,滚动窗口内存边界 |
| `tui/` | **`app.go` 的 `App` 是唯一 tea.Model**;`logview.go` 是其渲染层(视觉行/换行/ANSI);`ReplaceStream` 支持源热切换;`sourcepicker*.go` 源选择器,`sshpw.go` SSH 密码认证重连 |
| `frp/` | `tunnel.go` 拉起 frpc 子进程建 stcp visitor 隧道(要求 frp ≥ v0.52 TOML);`store.go` 持久化到 `~/.local/state/logview/frp.json`。`stream/frp.go` 的 FRPSource = 隧道 + SSH tail |
| `export/` `stacktrace/` `upgrade/` | 选中行导出;Java 堆栈折叠;自升级(brew 安装则委托 `brew upgrade`) |

### 配置与状态文件

- 配置:`~/.config/logview/rules.yaml`(首跑自动落盘默认配置;schema 见 `parser/rules.go` 的 `rulesFile`,含 patterns/rules/fields/theme/hides/ssh_hosts)。
- 运行时状态:`~/.local/state/logview/{frp.json, session.json, usage.json}`(usage 是源选择器 frecent 热点,7 天半衰期)。

## 测试风格

- TUI 测试是纯单元测试(不用 teatest):`newTestApp()` 直接构造 App + mockStream,直接调 `processLine`/`Update`,断言 `View()` 字符串输出。参考 `internal/tui/app_test.go` 头部。
- `internal/tui/main_test.go` 的 `TestMain` 把 HOME 重定向到临时目录,防止污染真实 usage.json/session.json——新写 tui 测试依赖此隔离。
- 共享测试 fixture 在 `testutil/`(如 `JavaLogbackLines()`)。

## 注意事项

- **搜索/grep 时务必排除 `.worktrees/`**——里面是同名副本 worktree,不排除则每个文件命中两遍。
- skill 产物(spec/plan/design)一律写入 vault 需求目录 `specs/`/`plans/`(命名 `YYYY-MM-DD-主题.md`),禁止写入代码仓库;历史 23 份已于 2026-09-04 迁入 vault 各需求目录,代码仓库 `docs/` 仅存 README 截图。
- 版本注入变量在 `main.go`(`main.version/commit/date`),Makefile 与 .goreleaser.yml 的 ldflags 必须一致。
