# time: 时间范围搜索 — 行为规格测试矩阵

> 状态:测试设计(2026-09-03)
> 关联 spec:docs/superpowers/specs/2026-08-28-time-range-search-design.md(语法/解析/匹配正本)
> 目标:可直接翻译成 `internal/tui/searchquery_test.go` 风格的表驱动用例(标准 go test,无外部断言库)

## 翻译约定(全矩阵通用)

- **anchor 注入**:spec 要求 `parseSearchQuery` 增加 anchor 参数,测试可注入固定时刻。矩阵记作 `parseSearchQueryWithAnchor(q, anchor)`(命名以实现为准);生产路径默认 `time.Now().UTC()`。
- **默认 anchor**:`A0 = 2026-08-28T10:00:00Z`。除特别标注外所有用例 anchor = A0。
- **行构造**:沿用 `parsedLineWithTime(...)` helper,时间一律 `time.Date(..., time.UTC)` 构造(spec:锚定全部 UTC,与 line.Time 无时区语义一致;旧测试用 `time.Local`,新用例必须改 UTC,否则时区敏感断言会漂移)。无时间字段行 = 不设 Time(IsZero),用 `parsedLine(...)` 构造。
- **期望语义**:
  - `命中 / 不命中` = `q.MatchLine(line)` 布尔结果。
  - `解析错误` = 产生 errorNode,断言三件套:① 不 panic;② 对**任意**行 `MatchLine = false`(空集);③ **不降级为关键字**——message 恰含错误子串的行也不命中。
- **表驱动骨架**(一节通用):

```go
type tc struct {
    query   string
    line    *model.ParsedLine
    want    bool
    wantErr bool // true → errorNode:两行验证(普通行 + message 含错误子串的行)均 false
}
```

---

## 一、time: 解析与匹配用例(核心)

### 1.1 值形态 × 方向符(23 条)

纯时分锚定 anchor 当天 00:00 + 时分;纯日期补全为当天 00:00:00(依据"补全规则同单边(时区/年锚定)");分精度秒补 0;`>` / `<` 严格不含恰等行,`>=` / `<=` 含;`_` 为 `T` 的宽容别名。

| # | 查询 | anchor | 测试行(Time,UTC) | 期望 | 断言要点 |
|---|---|---|---|---|---|
| A1 | `time:>9:00` | A0 | 08-28 09:30 | 命中 | 前导零宽容:9:00 ≡ 09:00 |
| A2 | `time:>09:00` | A0 | 08-28 09:30 | 命中 | 与 A1 行为等价 |
| A3 | `time:>9:00` | A0 | 08-28 09:00:00 | 不命中 | `>` 严格,恰等不含 |
| A4 | `time:>09:00` | A0 | 08-28 09:00:00 | 不命中 | 同 A3 |
| A5 | `time:>=9:00` | A0 | 08-28 09:00:00 | 命中 | `>=` 含边界,叠加前导零宽容 |
| A6 | `time:>=09:00` | A0 | 08-28 09:00:00 | 命中 | 同 A5 |
| A7 | `time:<10:00` | A0 | 08-28 09:59:00 | 命中 | |
| A8 | `time:<10:00` | A0 | 08-28 10:00:00 | 不命中 | `<` 严格 |
| A9 | `time:<=10:00` | A0 | 08-28 10:00:00 | 命中 | `<=` 含 |
| A10 | `time:<10:00:30` | A0 | 08-28 10:00:29 | 命中 | 秒粒度参与比较 |
| A11 | `time:<10:00:30` | A0 | 08-28 10:00:30 | 不命中 | `<` 严格(秒级) |
| A12 | `time:<=10:00:30` | A0 | 08-28 10:00:30 | 命中 | `<=` 含(秒级) |
| A13 | `time:>09:00:30` | A0 | 08-28 09:00:31 | 命中 | 时分秒形态 |
| A14 | `time:>09:00:30` | A0 | 08-28 09:00:30 | 不命中 | `>` 严格(秒级) |
| A15 | `time:>2026-08-26` | A0 | 08-26 08:00 | 命中 | 纯日期 → 当天 00:00:00 |
| A16 | `time:>2026-08-26` | A0 | 08-26 00:00:00 | 不命中 | `>` 严格(日期补零后恰等) |
| A17 | `time:>=2026-08-26` | A0 | 08-26 00:00:00 | 命中 | `>=` 含 |
| A18 | `time:>2026-08-26` | A0 | 08-25 23:59 | 不命中 | 前一天不满足 |
| A19 | `time:>2026-08-26T09:00` | A0 | 08-26 09:00:30 | 命中 | 分精度,秒补 0 → 界值 09:00:00 |
| A20 | `time:>2026-08-26T09:00` | A0 | 08-26 09:00:00 | 不命中 | 恰等(补零后)不含 |
| A21 | `time:>=2026-08-26T09:00:30` | A0 | 08-26 09:00:30 | 命中 | 完整 RFC3339 形态 + `>=` |
| A22 | `time:>2026-08-26_09:00:30` | A0 | 08-26 09:00:31 | 命中 | `_` 别名 ≡ `T` |
| A23 | `time:>2026-08-26_09:00:30` | A0 | 08-26 09:00:29 | 不命中 | `_` 别名按完整时间值比较 |

### 1.2 相对时长(13 条)

相对 → anchor - 时长;`-1h30m` 复合与 `-90m` 总量等价;`time:-10m` 简写默认 `>`。

| # | 查询 | anchor | 测试行(Time,UTC) | 期望 | 断言要点 |
|---|---|---|---|---|---|
| B1 | `time:>-10m` | A0 | 08-28 09:51 | 命中 | 锚点 = A0 - 10m = 09:50 |
| B2 | `time:>-10m` | A0 | 08-28 09:50 | 不命中 | `>` 严格,恰等不含 |
| B3 | `time:>=-10m` | A0 | 08-28 09:50 | 命中 | `>=` 含 |
| B4 | `time:-10m` | A0 | 08-28 09:55 | 命中 | 简写 ≡ `time:>-10m` |
| B5 | `time:-10m` | A0 | 08-28 09:45 | 不命中 | 简写默认 `>` 非 `>=` |
| B6 | `time:>-2d` | A0 | 08-26 10:00:01 | 命中 | d 单位;锚点 = 08-26 10:00 |
| B7 | `time:>-2d` | A0 | 08-26 09:59:00 | 不命中 | 早于锚点 |
| B8 | `time:>-1h30m` | A0 | 08-28 08:31 | 命中 | 复合时长;锚点 = 08:30 |
| B9 | `time:>-90m` | A0 | 08-28 08:31 | 命中 | 90m ≡ 1h30m(同 B8 行为) |
| B10 | `time:>-90m` | A0 | 08-28 08:29 | 不命中 | 早于 08:30 锚点 |
| B11 | `time:>-30s` | A0 | 08-28 09:59:31 | 命中 | s 单位;锚点 = 09:59:30 |
| B12 | `time:>-30s` | A0 | 08-28 09:59:30 | 不命中 | `>` 严格 |
| B13 | `time:>-90m` | A0 | (断言解析产物) | 锚点 == A0.Add(-90m) | anchor 减法正确性:经 TimeRangeHint 文本或节点导出字段直接断言绝对时刻 = 2026-08-28 08:30:00Z |

### 1.3 区间糖(12 条)

`值..值` 闭区间 = `>=下界 AND <=上界`,端点行命中;两侧各自按时间值规则补全(纯时分锚 anchor 当天)。

| # | 查询 | anchor | 测试行(Time,UTC) | 期望 | 断言要点 |
|---|---|---|---|---|---|
| C1 | `time:10:00..11:00` | A0 | 08-28 10:00:00 | 命中 | 闭区间下端点含 |
| C2 | `time:10:00..11:00` | A0 | 08-28 11:00:00 | 命中 | 闭区间上端点含 |
| C3 | `time:10:00..11:00` | A0 | 08-28 10:30 | 命中 | 区间内 |
| C4 | `time:10:00..11:00` | A0 | 08-28 09:59:59 | 不命中 | 早于下界 |
| C5 | `time:10:00..11:00` | A0 | 08-28 11:00:01 | 不命中 | 晚于上界 |
| C6 | `time:2026-08-26T09:00..2026-08-26T11:00` | A0 | 08-26 10:00 | 命中 | 带日期区间 |
| C7 | `time:2026-08-26T09:00..2026-08-26T11:00` | A0 | 08-26 08:59 | 不命中 | 早于下界 |
| C8 | `time:2026-08-26T09:00..2026-08-26T11:00` | A0 | 08-27 10:00 | 不命中 | 次日同时刻不在区间(区间不跨日) |
| C9 | `time:2026-08-26..2026-08-27` | A0 | 08-26 15:00 | 命中 | 日期区间;上界补全为 08-27 00:00:00 |
| C10 | `time:2026-08-26..2026-08-27` | A0 | 08-27 00:00:00 | 命中 | 上界端点恰等含(闭) |
| C11 | `time:2026-08-26..2026-08-27` | A0 | 08-27 23:59 | 不命中 | 超 08-27 00:00 上界 |
| C12 | `time:09:00..2026-08-29T10:00` | A0 | 08-28 09:30 | 命中 | 混合粒度:左纯时分锚 A0 当天(08-28 09:00),右带日期,两侧各自补全 |

### 1.4 显式双条件 AND 对照(3 条)

`time:>09:00 time:<10:00` 为隐式 AND,**开区间**——与闭区间糖的端点行为形成对照。

| # | 查询 | anchor | 测试行(Time,UTC) | 期望 | 断言要点 |
|---|---|---|---|---|---|
| D1 | `time:>09:00 time:<10:00` | A0 | 08-28 09:30 | 命中 | 隐式 AND 区间内 |
| D2 | `time:>09:00 time:<10:00` | A0 | 08-28 09:00:00 | 不命中 | 开区间下端不含(对照 C1 闭糖含) |
| D3 | `time:>09:00 time:<10:00` | A0 | 08-28 10:00:00 | 不命中 | 开区间上端不含(对照 C2) |

### 1.5 多时段与正交组合(11 条)

| # | 查询 | anchor | 测试行 | 期望 | 断言要点 |
|---|---|---|---|---|---|
| E1 | `time:10:00..11:00 OR time:13:00..14:00` | A0 | T=08-28 10:30 | 命中 | 第一段 |
| E2 | `time:10:00..11:00 OR time:13:00..14:00` | A0 | T=08-28 13:30 | 命中 | 第二段 |
| E3 | `time:10:00..11:00 OR time:13:00..14:00` | A0 | T=08-28 12:00 | 不命中 | 两段缝隙 |
| E4 | `time:10:00..11:00 OR time:13:00..14:00` | A0 | T=08-28 11:00:00 | 命中 | 段 1 闭端点 |
| E5 | `time:10:00..11:00 OR time:13:00..14:00` | A0 | T=08-28 14:00:00 | 命中 | 段 2 闭端点 |
| E6 | `level:ERROR AND time:>-30s` | A0 | level=ERROR,T=08-28 09:59:31 | 命中 | time: 与既有字段语法正交组合 |
| E7 | `level:ERROR AND time:>-30s` | A0 | level=INFO,T=08-28 09:59:31 | 不命中 | level 不符 |
| E8 | `level:ERROR AND time:>-30s` | A0 | level=ERROR,T=08-28 09:00 | 不命中 | 时间不符 |
| E9 | `NOT time:>09:00` | A0 | T=08-28 08:30 | 命中 | 有时间行的 NOT 常规取反 |
| E10 | `NOT time:>09:00` | A0 | T=08-28 09:30 | 不命中 | |
| E11 | `time:>09:00 OR level:ERROR` | A0 | level=WARN,T=08-28 08:00 | 命中 | OR 右支救活(行有时间,非 NULL 场景) |

### 1.6 非法输入 → errorNode(16 条)

断言模式:① 不 panic;② 空集(任意行 false);③ 不降级为关键字(防降级行 = message 恰含错误子串)。

| # | 查询 | anchor | 防降级测试行(message 含子串) | 期望 | 断言要点 |
|---|---|---|---|---|---|
| F1 | `time:09:00` | A0 | msg="job at 09:00 started" | 解析错误 | 缺方向符;若降级关键字则该行会命中 |
| F2 | `time:>8-26` | A0 | msg="backup 8-26 done" | 解析错误 | MM-DD 无年份格式已删 |
| F3 | `time:>9:0` | A0 | (任意行) | 解析错误 | 分钟必须两位(`H:MM`) |
| F4 | `time:>` | A0 | (任意行) | 解析错误 | 方向符后空值(isPartialQuery 层另测,见 O2) |
| F5 | `time:>10:00..` | A0 | msg="range 10:00.. open" | 解析错误 | 方向符后区间缺右值 |
| F6 | `time:10:00..` | A0 | (任意行) | 解析错误 | 区间缺右值 |
| F7 | `time:-10x` | A0 | msg="took -10x retries" | 解析错误 | 非法单位(白名单 s/m/h/d) |
| F8 | `time:>+10m` | A0 | (任意行) | 解析错误 | 正号拒绝(相对形态仅 `-` 开头) |
| F9 | `time:>-10` | A0 | (任意行) | 解析错误 | 纯数字缺单位 |
| F10 | `time:>09:00:3` | A0 | (任意行) | 解析错误 | 秒一位(对称 F3) |
| F11 | `time:>2026-8-26` | A0 | (任意行) | 解析错误 | 仅小时宽容;`YYYY-MM-DD` 月份须两位 |
| F12 | `time:>24:00` | A0 | (任意行) | 解析错误 | 小时越界;实现须显式校验 0-23,否则 time.Date 归一化为次日(该用例暴露归一化 bug) |
| F13 | `time:>09:60` | A0 | (任意行) | 解析错误 | 分钟越界;同 F12 须显式校验 0-59 |
| F14 | `time:>xx OR level:ERROR` | A0 | level=ERROR,任意时间 | 不命中(整体空集) | errorNode 出现在 OR 左支 → 整个查询匹配空集 |
| F15 | `NOT time:>xx` | A0 | T=08-28 08:30 | 不命中(整体空集) | errorNode 在 NOT 子树同样传播,不 panic |
| F16 | `level:ERROR AND time:>xx` | A0 | level=ERROR | 不命中(整体空集) | AND 右支传播 |

### 1.7 跨天锚定(6 条)

纯时分只锚 anchor 当天,比较统一为绝对时刻——修复旧"每日窗口"病。

| # | 查询 | anchor | 测试行(Time,UTC) | 期望 | 断言要点 |
|---|---|---|---|---|---|
| G1 | `time:>09:00` | A0 | 08-27 23:50 | 不命中 | 昨天 23:50 不算"今天 09:00 之后" |
| G2 | `time:>09:00` | A0 | 08-28 09:30 | 命中 | 今天 09:30 命中 |
| G3 | `time:>09:00` | A0 | 08-29 08:00 | 命中 | 明日 08:00 绝对时刻晚于今日 09:00 → 命中(绝对比较,非每日窗口) |
| G4 | `time:>09:00` | A0 | 08-28 09:00:00 | 不命中 | 跨天用例中的 `>` 严格边界 |
| G5 | `time:>=09:00` | A0 | 08-28 09:00:00 | 命中 | `>=` 含 |
| G6 | `time:<09:00` | A0 | 08-27 23:50 | 命中 | `<` 为绝对比较,昨天更早即命中,无跨天排除(与 G1 的 `>` 方向对照) |

### 1.8 IsZero 行 NULL 传播(8 条)

无时间字段行对任何 time 条件不命中;notNode 对 time 型直接子节点特判防翻转。

| # | 查询 | anchor | 测试行 | 期望 | 断言要点 |
|---|---|---|---|---|---|
| H1 | `time:>09:00` | A0 | IsZero,msg 任意 | 不命中 | 基础 NULL 语义 |
| H2 | `NOT time:>09:00` | A0 | IsZero,msg 任意 | 不命中 | **notNode 特判**:防止 !false = true 翻转(NULL 传播) |
| H3 | `level:ERROR AND time:>09:00` | A0 | IsZero,level=ERROR | 不命中 | AND 中 time 分支 false |
| H4 | `time:>09:00 OR level:ERROR` | A0 | IsZero,level=ERROR | 命中 | **分支级 NULL**:time 分支自身 false,另一支 true 救活。⚠ 裁决点:spec 措辞"含 NOT/OR 分支均不匹配"按分支级解读;若实现裁定查询级传播(任一 time 条件 → IsZero 恒 false),本条期望反转为"不命中",实现 PR 中须定案 |
| H5 | `time:10:00..11:00` | A0 | IsZero | 不命中 | 区间糖 = 两个 time 条件 AND |
| H6 | `time:>09:00 time:<10:00` | A0 | IsZero | 不命中 | 双条件同理 |
| H7 | `NOT (time:>09:00 OR level:ERROR)` | A0 | IsZero,level=INFO | 命中 | spec 字面:特判仅限 notNode 的**直接** time 型子节点;此处直接子节点是 orNode(false OR false = false),NOT → true。⚠ 如实现把特判加深到子树,本条反转为"不命中",与 H4 同步定案 |
| H8 | `time:>09:00 OR NOT time:<08:00` | A0 | IsZero | 不命中 | OR 右支为 NOT time → 特判 false,两支皆 false |

### 1.9 TimeRangeHint 新格式(6 条)

spec UI 段:显示解析后的绝对锚点;解析失败红字;OR/NOT 语境不显示区间 hint。断言用**子串包含**(具体排版以实现为准)。

| # | 查询 | anchor | 期望 hint | 断言要点 |
|---|---|---|---|---|
| I1 | `time:>-10m` | A0 | 含 `09:50:00` 且含 `-10m` | 相对值显示解析后绝对时刻 + 原始时长 |
| I2 | `time:10:00..11:00` | A0 | 含 `2026-08-28` 与 `10:00`、`11:00` | 区间显示补全后的绝对区间 |
| I3 | `time:>xx` | A0 | 含 `time 语法错误` | errorNode 红字提示 |
| I4 | `ERROR` | A0 | == "" | 纯关键字无 hint(现有行为回归) |
| I5 | `time:10:00..11:00 OR level:ERROR` | A0 | == "" | OR 语境不显示区间 hint |
| I6 | `NOT time:>09:00` | A0 | == "" | NOT 语境不显示区间 hint |

---

## 二、现有搜索语法回归用例(防破坏)

### 2.1 布尔 / 括号 / 引号 / NOT(15 条)

| # | 查询 | 测试行(level/message 等) | 期望 | 断言要点 |
|---|---|---|---|---|
| J1 | `(level:ERROR OR level:WARN) AND (message:timeout OR message:retry)` | level=ERROR,msg="connection timeout" | 命中 | 双括号嵌套组合,两各取其一 |
| J2 | 同 J1 | level=WARN,msg="db retry now" | 命中 | 另一组合 |
| J3 | 同 J1 | level=ERROR,msg="all good" | 不命中 | 右括号组不满足 |
| J4 | 同 J1 | level=INFO,msg="connection timeout" | 不命中 | 左括号组不满足 |
| J5 | `ERROR OR (WARN AND timeout)` | msg="ERROR here" | 命中 | 嵌套括号(现有 TestNestedParens 防破坏照录) |
| J6 | 同 J5 | msg="WARN timeout here" | 命中 | |
| J7 | 同 J5 | msg="WARN something else" | 不命中 | 括号内 AND 不满足 |
| J8 | `message:"hello world"` | msg="hello world again" | 命中 | 引号整短语(字段) |
| J9 | `message:"hello world"` | msg="hello there world" | 不命中 | 词序打散不算短语 |
| J10 | `"connection timeout"` | msg="connection timeout occurred" | 命中 | 引号整短语(裸关键字) |
| J11 | `message:ERROR NOT timeout` | msg="[ERROR] boom" | 命中 | NOT 排除 |
| J12 | 同 J11 | msg="[ERROR] connection timeout" | 不命中 | 含被排除词 |
| J13 | `level:ERROR AND timeout OR level:WARN` | level=WARN,msg="careful" | 命中 | AND 优先于 OR:WARN 单独走 OR 支 |
| J14 | 同 J13 | level=ERROR,msg="connection timeout" | 命中 | AND 支命中 |
| J15 | 同 J13 | level=ERROR,msg="all good" | 不命中 | AND 支不满足 |

### 2.2 字段匹配(6 条)

| # | 查询 | 测试行 | 期望 | 断言要点 |
|---|---|---|---|---|
| K1 | `level:ERROR` | level=ERROR | 命中 | 字段精确 |
| K2 | `level:ERROR` | level=error | 命中 | 大小写不敏感 |
| K3 | `level:ERROR` | level=INFO | 不命中 | |
| K4 | `traceId:abc` | traceId=abc123 | 命中 | 字段包含匹配 |
| K5 | `traceId:abc` | traceId=xyz789 | 不命中 | |
| K6 | `abc123` | traceId=abc123,msg 不含 abc123 | 不命中 | 裸词只搜 message(breaking 行为保持) |

### 2.3 裸 `>X` / `<X>` 字面量防劫持(5 条)

spec:**裸 `>X` / `<X`(无 time: 前缀)永不进时间语义**,作字面量关键字。

| # | 查询 | 测试行 | 期望 | 断言要点 |
|---|---|---|---|---|
| L1 | `>09:00` | msg="peak >09:00 traffic" | 命中 | 字面量关键字命中 message |
| L2 | `>09:00` | T=08-28 10:00,msg 不含 ">09:00" | 不命中 | 行时间满足也不进时间语义(防劫持核心断言) |
| L3 | `<init>` | msg="<init> startup phase" | 命中 | |
| L4 | `<2026-08-26T09:00>` | msg="ts <2026-08-26T09:00> in text" | 命中 | 日志内容含尖括号时间串不被劫持 |
| L5 | `<2026-08-26T09:00>` | T=08-25 12:00,msg 不含该串 | 不命中 | 同 L2 |

### 2.4 高亮关键词提取 HighlightKeywords()(4 条)

| # | 查询 | 期望 | 断言要点 |
|---|---|---|---|
| M1 | `level:ERROR AND time:>09:00` | == `[ERROR]`(恰 1 个) | time: 条件不产生高亮词:不含 "09:00"、不含 "time" |
| M2 | `time:>09:00` | 空列表 | 纯 time 查询无高亮 |
| M3 | `>09:00`(裸) | 含 `>09:00` | 裸字面量产生高亮词 |
| M4 | `time:10:00..11:00` | 空列表 | 区间糖同样不产生 |

### 2.5 after:/before: 移除兜底(4 条)

| # | 输入 | 测试行 | 期望 | 断言要点 |
|---|---|---|---|---|
| N1 | (直接断言 `fieldPrefixes`) | — | 含 `time:`,不含 `after:`/`before:` | 前缀表变更 |
| N2 | `after:09:00` | msg="after:09:00 checkpoint" | 命中 | 未知前缀 → 整词字面关键字 |
| N3 | `after:09:00` | T=08-28 10:00,msg 不含 "after:09:00" | 不命中 | 不进时间语义 |
| N4 | `before:10:00` | msg="before:10:00 cutoff" | 命中 | 同 N2 |

### 2.6 isPartialQuery 悬空中间态(15 条)

spec 扩展:以 `time:`、`time:>`、`time:>=` 等 time: 悬空前缀结尾视为中间态,不重过滤。

| # | 输入 | 期望 | 断言要点 |
|---|---|---|---|
| O1 | `time:` | true | 悬空前缀 |
| O2 | `time:>` | true | |
| O3 | `time:>=` | true | |
| O4 | `time:<` | true | 方向符家族("等"字) |
| O5 | `time:<=` | true | |
| O6 | `level:ERROR time:>` | true | 前缀结尾即可,不限整串 |
| O7 | `time:10:00..` | true | 区间打字中(注:直接 parse 此串则为 errorNode,见 F6,两断言并存不矛盾) |
| O8 | `time:>-` | true | 相对值打字中(宽容家族;若实现收窄到 spec 明举的三个,此条移除并在 PR 标注) |
| O9 | `time:>09:00` | false | 完整形态立即生效 |
| O10 | `time:10:00..11:00` | false | |
| O11 | `time:>-10m` | false | |
| O12 | `level:ERROR AND time:>09:00` | false | 组合完整 |
| O13 | `time:>=2026-08-26` | false | |
| O14 | 旧样本回归:`not` / `(` / `"hello` / `level:ERROR NOT` 仍 true;`ERROR` / `(ERROR OR WARN)` / `"hello world"` / `after:09:00` 仍 false | 各自期望 | 防扩展破坏既有中间态判定 |
| O15 | App 层:`a.searchInput = "time:"` → `a.currentQuery().IsEmpty()` 且匹配任意行 | 通过 | 应用层不因悬空 time: 重过滤/崩溃(参照 TestCurrentQueryPartialStripsOperator 断言风格) |

---

## 三、集成用例(file 模式多日,4 条)

fixture(`testutil` 新增多日样本,java-logback 格式,时间经 parser 解析为无时区即 UTC 语义):

```
2026-08-26 08:00:00.000  INFO   day26 early
2026-08-26 12:00:00.000  ERROR  day26 error
2026-08-26 23:59:00.000  WARN   day26 late
2026-08-27 08:59:00.000  INFO   day27 before-boundary
2026-08-27 09:00:00.000  INFO   day27 boundary-exact
2026-08-27 09:00:01.000  INFO   day27 boundary-plus-1s
2026-08-27 10:30:00.000  ERROR  day27 error
2026-08-27 23:00:00.000  INFO   day27 late
```

纯时分形态不进集成(依赖真实 Now 锚点,不稳定),由一节 anchor 注入覆盖;集成只测带完整日期形态(anchor 无关)。

| # | 查询 | 期望命中集合 | 断言要点 |
|---|---|---|---|
| P1 | `time:>2026-08-27T09:00` | 恰 3 行:day27 boundary-plus-1s / day27 error / day27 late | 不含任何 26 号行;不含 08:59;不含 boundary-exact(`>` 严格) |
| P2 | `time:2026-08-26..2026-08-27` | 恰 3 行:26 号全部 | 上界 = 08-27 00:00:00,27 号所有行(含 08:59)均不在内 |
| P3 | `level:ERROR AND time:>2026-08-27T09:00` | 恰 1 行:day27 error(08-27 10:30) | 26 号 ERROR 行被时间剪掉;组合过滤端到端 |
| P4 | `time:<2026-08-27T09:00` | 恰 4 行:26 号 3 行 + day27 before-boundary | `<` 方向跨日端到端 |

集成测试方式:file 模式注入 fixture(参照 app_test.go 的 mockStream/newTestApp 基建),断言可见行数与行内容子集。

---

## 四、迁移注记(现有测试需同步改写)

- `TestTimeFieldRemoved`(searchquery_test.go:267):断言 `time:` 不在 fieldPrefixes、`time:09:30` 按关键字处理——与本设计**直接冲突**,需改写为:fieldPrefixes 含 `time:`;`time:09:30` 为 errorNode(F1)。其余"裸词只搜 message"部分保留。
- `TestTimeRange`(:139)/ `TestTimeRangeWithKeyword`(:170)/ `TestTimeRangeHint`(:200):基于 `after:`/`before:` 旧语法,需迁移为 `time:` 等价形态(时间构造改 UTC)或删除后由本矩阵一节覆盖。
- `TestIsPartialQuery`(:417):补 O1-O13 样本;现 falses 中 `"after:09:00"` 保留(N2 兜底语义)。
- `parseSearchQuery` 签名增加 anchor 参数:存量单参调用点包装默认 `time.Now().UTC()`,旧测试不改语义。

## 五、统计

| 节 | 组 | 数量 |
|---|---|---|
| 一 | 1.1 值形态 × 方向符 | 23 |
| 一 | 1.2 相对时长 | 13 |
| 一 | 1.3 区间糖 | 12 |
| 一 | 1.4 显式双条件 AND | 3 |
| 一 | 1.5 多时段与正交组合 | 11 |
| 一 | 1.6 非法输入 errorNode | 16 |
| 一 | 1.7 跨天锚定 | 6 |
| 一 | 1.8 IsZero NULL 传播 | 8 |
| 一 | 1.9 TimeRangeHint | 6 |
| 二 | 2.1 布尔/括号/引号/NOT | 15 |
| 二 | 2.2 字段匹配 | 6 |
| 二 | 2.3 裸字面量防劫持 | 5 |
| 二 | 2.4 高亮提取 | 4 |
| 二 | 2.5 after/before 兜底 | 4 |
| 二 | 2.6 isPartialQuery | 15 |
| 三 | 集成(file 多日) | 4 |
| **合计** | | **151** |

两个实现前须定案的裁决点:H4(OR 分支级 vs 查询级 NULL 传播)与联动的 H7;O8(`time:>-` 是否纳入中间态)。
