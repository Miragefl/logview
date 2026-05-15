package testutil

func JavaLogbackLines() []string {
	return []string{
		"2026-05-15 09:27:01.130 [main] [abc123] INFO  com.example.App - hello world",
		"2026-05-15 09:27:02.000 [main] [abc123] WARN  com.example.App - something odd",
		"2026-05-15 09:27:03.000 [worker-1] [def456] ERROR com.example.App - java.lang.NullPointerException",
		"2026-05-15 09:27:03.001 [worker-1] [def456]   at com.example.App.run(App.java:42)",
		"2026-05-15 09:27:03.002 [worker-1] [def456]   at com.example.App.process(App.java:10)",
		"2026-05-15 09:27:04.000 [main] [abc123] INFO  com.example.App - done",
	}
}