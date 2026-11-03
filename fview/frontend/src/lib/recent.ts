// lib/recent.ts 最近源直达：四类键格式拆解 + openRecent 分派 + recentLabel 展示
// （Welcome/LocalPanel 共用正本，spec 2026-11-03-pass-search-recent）。
// v2 键：file:{p} / ssh:{host}:{p} / frp:{conn}:{p} / k8sres:{ctx}/{ns}/{kind}/{name}；
// 旧格式（缺 path/ctx）no-op——usage 7 天半衰期自清，不迁移（守则：未部署零兼容层）。
import { getSavedPass, openFileTab, openFRPTab, openK8sTab, openSSHTab, usageTouch } from './backend'
import { useUiStore } from '../stores/ui'

/** recentLabel 欢迎页展示名：file→basename；ssh/frp→`{host} · {basename}`；k8sres→`{kind}/{name} · {ns}`；旧格式原样。 */
export function recentLabel(k: string): string {
  if (k.startsWith('file:')) return k.slice(5).replace(/.*\//, '') || k
  const m = k.match(/^ssh:(.+):(\/.+)$/)
  if (m) return `${m[1]} · ${m[2].replace(/.*\//, '')}`
  const f = k.match(/^frp:(.+):(\/.+)$/)
  if (f) return `${f[1]} · ${f[2].replace(/.*\//, '')}`
  const k8 = k.match(/^k8sres:([^/]+)\/([^/]+)\/([^/]+)\/([^/]+)$/)
  if (k8) return `${k8[3]}/${k8[4]} · ${k8[2]}`
  return k
}

/** openRecent 点击分派（含 usageTouch 热点；失败静默——源可能已不可达）。
 *  ssh/frp 贪婪切分（路径以 / 锚定，user@host:port 场景 host 含冒号也能正确切）；
 *  有存密码直接开 tab，无密码弹全局 BrowseDialog（initPath 落父目录）走密码流。 */
export async function openRecent(k: string): Promise<void> {
  try {
    if (k.startsWith('file:')) {
      await openFileTab(k.slice(5), 0, true)
      await usageTouch(k)
      return
    }
    const m = k.match(/^ssh:(.+):(\/.+)$/)
    if (m) {
      const pass = await getSavedPass(`ssh:${m[1]}`)
      if (pass) {
        await openSSHTab(m[1], m[2], 0, pass)
        await usageTouch(k)
      } else {
        useUiStore().openBrowse({ mode: 'ssh', host: m[1], initPath: m[2] })
      }
      return
    }
    const f = k.match(/^frp:(.+):(\/.+)$/)
    if (f) {
      const pass = await getSavedPass(`frp:${f[1]}`)
      if (pass) {
        await openFRPTab(f[1], f[2], 0, pass)
        await usageTouch(k)
      } else {
        useUiStore().openBrowse({ mode: 'frp', connName: f[1], initPath: f[2] })
      }
      return
    }
    // 旧格式降级（验收反馈②）：无路径的 ssh:/frp: 键信息仍有效——弹对应目录浏览
    // 落根目录，点击无反应是最差体验（k8sres 旧键缺 ctx/ns 无法定位，维持 no-op）
    const so = k.match(/^ssh:([^:]+)$/)
    if (so) {
      useUiStore().openBrowse({ mode: 'ssh', host: so[1] })
      return
    }
    const fo = k.match(/^frp:([^:]+)$/)
    if (fo) {
      useUiStore().openBrowse({ mode: 'frp', connName: fo[1] })
      return
    }
    const k8 = k.match(/^k8sres:([^/]+)\/([^/]+)\/([^/]+)\/([^/]+)$/)
    if (k8) {
      await openK8sTab(`${k8[3]}/${k8[4]}`, k8[2], k8[1], 0)
      await usageTouch(k)
    }
  } catch {
    /* 打开失败静默（连接不可达/文件已移动） */
  }
}
