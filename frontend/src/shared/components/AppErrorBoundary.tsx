import { Component, type PropsWithChildren } from 'react'

// Failed lazy chunks (including stale tabs after a release/rollback) must not
// unmount the whole workspace into an unexplained blank screen.
export class AppErrorBoundary extends Component<PropsWithChildren, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <main role="alert" style={{ padding: 32, maxWidth: 640, margin: '64px auto' }}>
        <h2>页面暂时无法加载</h2>
        <p>服务可能刚完成更新，或网络连接中断。请刷新页面后继续，已保存的数据不会丢失。</p>
        <button type="button" onClick={() => window.location.reload()}>刷新页面</button>
      </main>
    )
  }
}
