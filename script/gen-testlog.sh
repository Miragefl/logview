#!/usr/bin/env bash
# gen-testlog.sh —— 生成 fview/logview 验收用测试日志（java-logback 格式）。
# 用法：script/gen-testlog.sh [输出路径]   默认 ~/Documents/test-app.log
# 内容特征：多级别分布(INFO/WARN/ERROR/DEBUG)、traceId+耗时字段、多行 Java 堆栈、超长 JSON 行。
set -euo pipefail

OUT="${1:-$HOME/Documents/test-app.log}"
LINES="${LINES:-5200}"

awk -v total="$LINES" 'BEGIN {
  srand(42)
  # 消息池（模拟订单/支付/缓存/DB/网关五个模块）
  split("OrderService PayService CacheLoader UserDao GatewayFilter AuthInterceptor MsgConsumer HealthChecker", mods, " ")
  split("order created order paid order shipped payment callback received cache miss cache loaded db query slow redis ping ok token refreshed message consumed health-check ok retry scheduled task done batch import finished", msgs, " ")
  split("http-nio-8080-exec-1 http-nio-8080-exec-2 sched-pool-1 kafka-consumer-1 main timer", threads, " ")
  hh = 10; mm = 0; ss = 0; ms = 0
  for (i = 1; i <= total; i++) {
    # 时间推进：每行 +37~143ms
    ms += int(37 + rand() * 106)
    if (ms >= 1000) { ss += int(ms / 1000); ms %= 1000 }
    if (ss >= 60) { mm += int(ss / 60); ss %= 60 }
    if (mm >= 60) { hh += int(mm / 60); mm %= 60 }
    t = sprintf("2026-10-30 %02d:%02d:%02d.%03d", hh, mm, ss, ms)
    th = threads[int(rand() * 6) + 1]
    mod = mods[int(rand() * 9) + 1]
    msg = msgs[int(rand() * 20) + 1]
    # 级别分布：INFO 60% / DEBUG 15% / WARN 15% / ERROR 10%
    r = rand()
    lvl = (r < 0.60) ? "INFO " : (r < 0.75) ? "DEBUG" : (r < 0.90) ? "WARN " : "ERROR"
    # traceId/耗时：六成行携带
    extra = (rand() < 0.6) ? sprintf(" traceId=%08x cost=%dms", int(rand()*4294967295), int(rand()*900)) : ""
    logger = sprintf("c.example.%s", mod)
    printf "%s [%s] %s %-28s - %s%s\n", t, th, lvl, logger, msg, extra
    # 每约 500 行：一个 ERROR 附多行堆栈（8~14 帧）
    if (i % 500 == 0) {
      print "java.sql.SQLException: Connection refused: jdbc:mysql://10.0.1.7:3306/orderdb"
      frames = int(rand() * 7) + 8
      for (f = 1; f <= frames; f++) {
        printf "\tat com.example.%s.handle(%s.java:%d)\n", mods[int(rand()*9)+1], mods[int(rand()*9)+1], int(rand()*900) + 10
      }
      print "\tat org.springframework.web.filter.OncePerRequestFilter.doFilter(OncePerRequestFilter.java:117)"
    }
    # 每约 1000 行：一条超长 JSON payload（压测虚拟滚动横向滚动/单行不换行）
    if (i % 1000 == 0) {
      payload = ""
      for (j = 0; j < 120; j++) {
        payload = payload sprintf("\"f%d\":\"v-%04d-%s\",", j, j, (rand() < 0.5 ? "alpha" : "beta"))
      }
      printf "%s [http-nio-8080-exec-1] WARN  %-28s - full payload={%s\"end\":true}\n", t, "c.example.GatewayFilter", payload
    }
  }
}' > "$OUT"

wc -l "$OUT" | awk '{printf "生成完成：%s 共 %d 行\n", "'"$OUT"'", $1}'
