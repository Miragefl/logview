<script lang="ts">
  // 行详情侧滑面板：七字段表格 + 原文等宽展示 + 复制。
  import type { LineDTO } from '../lib/stores/tabs'

  export let line: LineDTO | null = null
  export let onClose: () => void = () => {}

  const FIELDS: Array<{ key: keyof LineDTO; label: string }> = [
    { key: 'seq', label: '序号' },
    { key: 'time', label: '时间' },
    { key: 'level', label: '级别' },
    { key: 'thread', label: '线程' },
    { key: 'traceId', label: 'TraceID' },
    { key: 'logger', label: 'Logger' },
    { key: 'message', label: '消息' },
  ]

  async function copyRaw() {
    if (line) await navigator.clipboard.writeText(line.raw)
  }
</script>

<aside class="detail">
  <div class="head">
    <span>行详情</span>
    <div>
      <button class="mini" on:click={copyRaw}>复制原文</button>
      <button class="mini" on:click={onClose}>×</button>
    </div>
  </div>
  {#if line}
    <table>
      {#each FIELDS as f (f.key)}
        <tr>
          <th>{f.label}</th>
          <td>{String(line[f.key])}</td>
        </tr>
      {/each}
    </table>
    <div class="raw">
      <div class="raw-label">原文</div>
      <pre>{line.raw}</pre>
    </div>
  {/if}
</aside>

<style>
  .detail {
    width: 360px;
    flex-shrink: 0;
    border-left: 1px solid #1c2836;
    background: #0c1119;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    font-size: 14px;
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 10px;
    color: #8b98a9;
    border-bottom: 1px solid #1c2836;
  }
  .head > div {
    display: flex;
    gap: 4px;
  }
  .mini {
    background: #1c2836;
    color: #8b98a9;
    border: 1px solid #2a3a4f;
    border-radius: 4px;
    cursor: pointer;
    padding: 2px 8px;
    font-size: 13px;
  }
  table {
    width: 100%;
    border-collapse: collapse;
  }
  th {
    text-align: left;
    color: #46586e;
    padding: 4px 10px;
    width: 70px;
    vertical-align: top;
    font-weight: normal;
  }
  td {
    padding: 4px 10px 4px 0;
    color: #c5cdd8;
    word-break: break-all;
  }
  tr {
    border-bottom: 1px solid #141d29;
  }
  .raw {
    padding: 8px 10px;
    flex: 1;
  }
  .raw-label {
    color: #46586e;
    margin-bottom: 4px;
  }
  pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
    font-family: 'SF Mono', Menlo, monospace;
    color: #c5cdd8;
    background: #10161f;
    padding: 8px;
    border-radius: 6px;
  }
</style>
