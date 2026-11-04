// vitest 全局 setup：node ≥ 22 的 experimental localStorage getter 会遮蔽
// jsdom 环境注入的 localStorage（读到 undefined），显式兜底一个简易实现。
const g = globalThis as { localStorage?: Storage }
if (g.localStorage === undefined) {
  const store = new Map<string, string>()
  const storage: Storage = {
    getItem: (k) => (store.has(k) ? store.get(k)! : null),
    setItem: (k, v) => void store.set(k, String(v)),
    removeItem: (k) => void store.delete(k),
    clear: () => store.clear(),
    key: (i) => [...store.keys()][i] ?? null,
    get length() {
      return store.size
    },
  }
  Object.defineProperty(globalThis, 'localStorage', {
    value: storage,
    configurable: true,
    writable: true,
  })
}

// jsdom 无 ResizeObserver：LogView 视口跟踪需要，给空 stub（布局尺寸测试走纯函数）
if (typeof globalThis.ResizeObserver === 'undefined') {
  class ResizeObserverStub {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
  ;(globalThis as unknown as { ResizeObserver: typeof ResizeObserverStub }).ResizeObserver =
    ResizeObserverStub
}

// jsdom 无 scrollIntoView：键盘导航跟随滚动(SearchPopup/BrowseDialog/书签列表)给空 stub
if (typeof Element.prototype.scrollIntoView !== 'function') {
  Element.prototype.scrollIntoView = function scrollIntoViewStub() {}
}
// jsdom 无 Element.scrollTo：naive SelectMenu 展开时 scrollToPendingNode 滚虚列表用
if (typeof Element.prototype.scrollTo !== 'function') {
  Element.prototype.scrollTo = function scrollToStub() {}
}

// jsdom 无 matchMedia：naive/vueuc 虚拟列表 touch 检测用(NSelect 展开即渲染 VirtualList),
// 给桌面态 stub(matches:false)——NSelect 聚焦守卫测试触发下拉展开所需
if (typeof window.matchMedia !== 'function') {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener() {},
      removeListener() {},
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent: () => false,
    }) as MediaQueryList
}
