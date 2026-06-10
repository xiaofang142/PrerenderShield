import React from 'react'
import { Card } from 'antd'

const SettingsPage: React.FC = () => {
  return (
    <div>
      <h1 className="page-title">系统设置</h1>
      <Card className="card">
        <p>系统设置页面开发中。支持的 API 端点：</p>
        <ul>
          <li>获取配置 (GET /api/v1/system/config)</li>
          <li>更新配置 (POST /api/v1/system/config)</li>
        </ul>
      </Card>
    </div>
  )
}

export default SettingsPage
