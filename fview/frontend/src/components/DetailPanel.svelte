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
    border-left: 1px solid #e2e2d8;
    background: #ebebe2;
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
    color: #6b7280;
    border-bottom: 1px solid #e2e2d8;
  }
  .head > div {
    display: flex;
    gap: 4px;
  }
  .mini {
    background: #e2e2d8;
    color: #6b7280;
    border: 1px solid #c7c7bc;
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
    color: #9aa0a8;
    padding: 4px 10px;
    width: 70px;
    vertical-align: top;
    font-weight: normal;
  }
  td {
    padding: 4px 10px 4px 0;
    color: #3a3f47;
    word-break: break-all;
  }
  tr {
    border-bottom: 1px solid #ecece4;
  }
  .raw {
    padding: 8px 10px;
    flex: 1;
  }
  .raw-label {
    color: #9aa0a8;
    margin-bottom: 4px;
  }
  pre {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
    font-family: 'SF Mono', Menlo, monospace;
    color: #3a3f47;
    background: #ffffff;
    padding: 8px;
    border-radius: 6px;
  }
</style>
