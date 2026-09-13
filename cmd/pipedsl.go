package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/creack/pty"
	"github.com/justfun/logview/internal/model"
	"github.com/justfun/logview/internal/stream"
	"golang.org/x/sys/unix"
)

// splitPipeSegments 把管道 DSL 参数按裸 | 分段:单/双引号内的 | 不拆,\ 转义。
// 段保留原文(含引号与空白),供首段 token 化或后续段原样拼 sh -c。
func splitPipeSegments(s string) ([]string, error) {
	var segs []string
	var cur strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			cur.WriteRune(r)
			escaped = true
		case quote != 0:
			cur.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			cur.WriteRune(r)
			quote = r
		case r == '|':
			segs = append(segs, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("管道表达式引号未闭合: %s", s)
	}
	if escaped {
		return nil, fmt.Errorf("管道表达式以转义符结尾: %s", s)
	}
	return append(segs, cur.String()), nil
}

// splitShellArgs shell 风格 token 化(空格分隔,单/双引号成组并剥壳,\ 转义下一字符)。
func splitShellArgs(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	var quote rune
	inToken := false
	escaped := false
	flush := func() {
		if inToken {
			args = append(args, cur.String())
			cur.Reset()
			inToken = false
		}
	}
	for _, r := range s {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
			inToken = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inToken = true
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
			inToken = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("参数引号未闭合: %s", s)
	}
	if escaped {
		return nil, fmt.Errorf("参数以转义符结尾: %s", s)
	}
	flush()
	return args, nil
}

// firstSegmentSource 首段(经 expandTailArgs 与 token 化)构造 logview 源:
// 裸路径→tail 语义;tail/file 子命令形式分别构造;其余子命令不支持。
func firstSegmentSource(tokens []string, history int) (stream.LogStream, error) {
	tokens = expandTailArgs(tokens)
	switch {
	case len(tokens) == 0:
		return nil, fmt.Errorf("管道 DSL 首段为空")
	case tokens[0] == "tail":
		args := tokens[1:]
		follow := 0
		explicitTail := false // --tail N 已显式指定行数时,-f 不再用配置 history 覆盖(-100f 语义)
		var paths []string
		for i := 0; i < len(args); i++ {
			if args[i] == "-f" {
				if !explicitTail {
					follow = history // 与 tail 子命令一致:仅 -f 时用配置尾行数
				}
			} else if strings.HasPrefix(args[i], "-") && len(args[i]) > 1 && args[i][1] >= '0' && args[i][1] <= '9' {
				// -Nf 已被 expandTailArgs 展开成 --tail N -f,此处兜底忽略
				continue
			} else if args[i] == "--tail" && i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil && n > 0 {
					follow = n
					explicitTail = true
				}
				i++
			} else {
				paths = append(paths, args[i])
			}
		}
		if len(paths) == 0 {
			return nil, fmt.Errorf("管道 DSL 首段缺少文件路径")
		}
		return stream.NewTailSource(paths, follow), nil
	case tokens[0] == "file":
		if len(tokens) < 2 {
			return nil, fmt.Errorf("file 子命令缺少路径")
		}
		return stream.NewFileSource(tokens[1:]), nil
	case tokens[0] == "pipe" || tokens[0] == "picker" || tokens[0] == "k8s" ||
		tokens[0] == "upgrade" || tokens[0] == "version" || tokens[0] == "completion":
		return nil, fmt.Errorf("管道 DSL 首段仅支持 文件路径/tail/file,不支持 %s", tokens[0])
	default:
		return stream.NewTailSource(tokens, history), nil // 裸路径(可多个)
	}
}

// pipeCmdSource 管道 DSL 数据源:首段源行喂 sh -c 命令链,stdout 行与退出提示行合流。
type pipeCmdSource struct {
	first stream.LogStream
	cmd   string
	ch    chan model.RawLine
	seq   atomic.Uint64
}

// newPipeCmdSourceCmd 构造管道命令源(first=首段源,cmdString=后续段以 | 拼接的命令链)。
func newPipeCmdSourceCmd(first stream.LogStream, cmdString string) *pipeCmdSource {
	return &pipeCmdSource{first: first, cmd: cmdString}
}

func (p *pipeCmdSource) Label() string { return p.first.Label() + " | " + p.cmd }

// Scope 词频隔离域沿用 pipe 语义(全局)。
func (p *pipeCmdSource) Scope() string { return "" }

func (p *pipeCmdSource) Start(ctx context.Context) (<-chan model.RawLine, error) {
	// pty 桥:命令 stdout/stderr 接伪终端 slave → grep/sed/awk 判定 stdout 为 TTY,
	// 自动行缓冲,follow 模式下新行实时流出(管道 stdout 是 4-64KB 全缓冲,行会滞留)。
	// stdin 仍走管道:pty 无法向子进程传 stdin EOF(关 master 等于 SIGHUP 整条管道,
	// 有丢尾行竞态),保留管道才能让首段读尽后命令干净退出(exit 0,不打退出提示行)。
	master, tty, err := pty.Open()
	if err != nil {
		return nil, err
	}
	// OPOST 关:子进程输出的 \n 原样到达 master,不被翻成 \r\n(源头修复,免后置剥 \r);
	// ECHO/ICANON 关:slave 输入侧本无人使用(防回显双保险)。c_cc 全零只影响输入侧
	// (VMIN/VTIME),对本源不生效——stdin 不经 pty。
	tio := &unix.Termios{Iflag: 0, Oflag: 0, Cflag: unix.CREAD | unix.CS8, Lflag: 0}
	if err := unix.IoctlSetTermios(int(tty.Fd()), ioctlSetTermios, tio); err != nil {
		master.Close()
		tty.Close()
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", p.cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		master.Close()
		tty.Close()
		return nil, err
	}
	cmd.Stdout = tty // *os.File 直接传 fd,Wait 无 stdout 拷贝协程依赖
	cmd.Stderr = tty
	if err := cmd.Start(); err != nil {
		master.Close()
		tty.Close()
		return nil, err
	}
	tty.Close() // 父进程弃用自己的 slave 副本:子进程退出、slave 全关后 master 读端即见 EOF/EIO
	p.ch = make(chan model.RawLine, 256)

	// 桥:首段源行 → 命令 stdin(首段通道关闭即管道 EOF,下游命令自然收尾)
	firstCh, err := p.first.Start(ctx)
	if err != nil {
		master.Close()
		return nil, err
	}
	go func() {
		defer stdin.Close()
		for raw := range firstCh {
			if _, err := fmt.Fprintln(stdin, raw.Text); err != nil {
				return
			}
		}
	}()

	// 命令输出(slave → master)行 → TUI
	go func() {
		defer master.Close()
		scanner := bufio.NewScanner(master)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			p.send(ctx, scanner.Text())
		}
		// 退出提示行(非零退出才提示;0 退出为正常 EOF,静默)
		if err := cmd.Wait(); err != nil {
			p.send(ctx, fmt.Sprintf("[logview] 管道命令退出: %v", err))
		}
		close(p.ch)
	}()
	return p.ch, nil
}

func (p *pipeCmdSource) send(ctx context.Context, text string) {
	select {
	case p.ch <- model.RawLine{Text: text, Source: "pipe", Seq: p.seq.Add(1)}:
	case <-ctx.Done():
	}
}

func (p *pipeCmdSource) Cleanup() error { return p.first.Cleanup() }

// runPipeDSL 解析并执行管道 DSL(Execute 的 DSL 分支入口)。
func runPipeDSL(dsl string) error {
	segs, err := splitPipeSegments(dsl)
	if err != nil {
		return err
	}
	if len(segs) < 2 {
		return fmt.Errorf("管道 DSL 需要至少两段(用 | 连接): %s", dsl)
	}
	firstTokens, err := splitShellArgs(segs[0])
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	first, err := firstSegmentSource(firstTokens, cfg.history)
	if err != nil {
		return err
	}
	src := newPipeCmdSourceCmd(first, strings.Join(segs[1:], "|"))
	return runTUI(src, false, cfg, false)
}

// pipeDSLArg 无子命令时,恰好一个含 | 的位置参数(非 flag)→ 返回之;否则空。
func pipeDSLArg(args []string) string {
	var positional []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
		}
	}
	if len(positional) == 1 && strings.Contains(positional[0], "|") {
		return positional[0]
	}
	return ""
}
