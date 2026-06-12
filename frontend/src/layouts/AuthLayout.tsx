import { Card, Typography } from 'antd'
import { Outlet } from 'react-router-dom'

const chain = [
  { label: '标讯大厅 · 发现商机', ai: false },
  { label: '招标文件解析', ai: true },
  { label: '分步生成标书', ai: true },
  { label: '合规检查 · 四层校验', ai: true },
  { label: '审批提交 · 开标结果', ai: false },
  { label: '中标案例回流知识库', ai: false },
]

export function AuthLayout() {
  return (
    <div className="auth-layout">
      <aside className="auth-brand-panel">
        <div>
          <div className="auth-brand-head">
            <div className="seal-mark seal-lg">标</div>
            <div>
              <Typography.Text className="auth-brand-name" style={{ color: 'inherit' }}>
                智标通
              </Typography.Text>
              <span className="auth-brand-en">ZhiBiaoTong</span>
            </div>
          </div>
          <p className="auth-thesis">
            让每一份标书，
            <br />
            都经得起<em>开标</em>。
          </p>
        </div>
        <ul className="auth-chain">
          {chain.map((step) => (
            <li key={step.label} className={step.ai ? 'ai-step' : undefined}>
              {step.label}
            </li>
          ))}
        </ul>
      </aside>
      <main className="auth-form-panel">
        <Card className="auth-card" variant="borderless">
          <Outlet />
        </Card>
      </main>
    </div>
  )
}
