import React from 'react'
import { Card } from 'antd'

const SSLPage: React.FC = () => {
  return (
    <div>
      <h1 className="page-title">SSL 证书管理</h1>
      <Card className="card">
        <p>SSL 证书管理页面开发中。支持的 API 端点：</p>
        <ul>
          <li>申请证书 (POST /api/v1/ssl/certificates)</li>
          <li>续签证书 (POST /api/v1/ssl/certificates/:domain/renew)</li>
          <li>删除证书 (DELETE /api/v1/ssl/certificates/:domain)</li>
          <li>通配符证书 (POST /api/v1/ssl/certificates/wildcard)</li>
          <li>证书列表 (GET /api/v1/ssl/certificates)</li>
        </ul>
      </Card>
    </div>
  )
}

export default SSLPage
