# Prerender Shield 功能说明

## 修复问题

### 1. 端口保存问题
- **问题**：添加站点时端口信息无法保存，默认80端口
- **修复**：在表单提交时将端口值添加到`siteData`对象中，并转换为整数类型
- **实现**：`Port: parseInt(values.port, 10) || 80`
- **位置**：`web/src/pages/Sites/Sites.tsx:627`

### 2. 预览跳转问题
- **问题**：点击预览时跳转的URL没有带端口
- **修复**：添加端口判断逻辑，80端口不拼接，其他端口需要拼接
- **实现**：
  ```javascript
  const url = port === 80 
    ? `http://${site.domain}` 
    : `http://${site.domain}:${port}`;
  ```
- **位置**：`web/src/pages/Sites/Sites.tsx:327-329`

### 3. 静态资源下载问题
- **问题**：点击下载静态资源时提示正在下载，但实际无反应
- **修复**：实现实际的下载逻辑，创建临时下载链接并触发点击
- **实现**：
  ```javascript
  const downloadLink = document.createElement('a');
  downloadLink.href = `/api/sites/${currentSite?.name}/static${file.path}`;
  downloadLink.download = file.name;
  downloadLink.target = '_blank';
  document.body.appendChild(downloadLink);
  downloadLink.click();
  document.body.removeChild(downloadLink);
  ```
- **位置**：`web/src/pages/Sites/Sites.tsx:447-453`

## 一个端口一个站点的设计

### 设计理念
- 每个站点对应一个独立的HTTP服务器实例
- 每个实例监听不同的端口
- 支持两种访问模式：直接对外访问和作为反向代理上游

### 后端实现
- 为每个站点启动独立的HTTP服务器实例
- 移除域名验证逻辑，允许任何域名访问
- 支持基于请求类型的处理逻辑：
  - 资源请求（.js, .css, .png等）：直接返回
  - 非资源请求：基于配置返回预渲染结果
- 支持上游代理模式，可以将请求转发到上游服务

### 前端实现
- 添加站点访问配置部分
- 提供两种访问模式选择：
  - 直接对外访问：站点直接处理外部请求
  - 作为反向代理上游：站点通过nginx等反向代理服务器访问
- 提供上游代理配置，包括启用开关和目标URL输入框

### nginx配置示例
```nginx
server {
    listen 80;
    server_name example.com;
    
    location / {
        proxy_pass http://127.0.0.1:89;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## 访问模式和上游代理

### 访问模式
- **直接对外访问**：站点直接处理外部请求，适合独立部署的情况
- **作为反向代理上游**：站点不直接暴露给外部，而是通过nginx等反向代理服务器访问，适合需要nginx处理域名解析和负载均衡的情况

### 上游代理
- **功能**：当启用上游代理时，prerender-shield会将收到的请求转发到配置的目标URL
- **使用场景**：
  - 已有运行中的Web服务，想要添加预渲染功能
  - 需要在多个服务之间共享prerender-shield的预渲染功能
  - 需要使用nginx等反向代理服务器处理域名解析
- **工作流程**：
  - 用户请求 → nginx（反向代理） → prerender-shield（端口89） → 上游服务（目标URL）
- **配置方法**：
  - 在新增或修改站点时，选择“站点访问配置”
  - 开启“启用上游代理”开关
  - 输入上游服务的URL，例如：http://127.0.0.1:8080

### 两者的配合使用

| 访问模式 | 启用上游代理 | 效果 | 适合场景 |
|---------|------------|------|---------|
| 直接对外访问 | 启用 | 站点直接接收外部请求，资源请求直接返回，非资源请求返回预渲染结果并转发到上游服务 | 已有运行中的Web服务，想要添加预渲染功能，无需修改原有服务的代码 |
| 直接对外访问 | 禁用 | 站点直接接收外部请求，资源请求直接返回，非资源请求返回预渲染结果，不转发到其他服务 | prerender-shield作为独立的HTTP服务器，处理所有请求 |
| 作为反向代理上游 | 启用 | 站点通过nginx接收请求，资源请求直接返回，非资源请求返回预渲染结果并转发到上游服务 | 使用nginx处理域名解析和负载均衡，prerender-shield处理预渲染，然后转发到后端服务 |
| 作为反向代理上游 | 禁用 | 站点通过nginx接收请求，资源请求直接返回，非资源请求返回预渲染结果，不转发到其他服务 | prerender-shield作为静态资源服务器，处理预渲染和静态资源请求 |

## UI交互改进

### 表单布局优化
- 使用卡片布局将表单分为基本信息、站点访问配置、预渲染配置、防火墙配置和SSL配置等模块
- 使用Row和Col组件实现更紧凑的表单布局，提高页面利用率

### 表单验证增强
- 站点名称：长度2-50字符，仅允许字母、数字、下划线和连字符
- 域名：必须是有效的URL格式
- 端口：必须是1-65535之间的数字
- 上游服务URL：仅当启用上游代理时必填

### 提交状态提示
- 增加加载状态提示，使用Modal组件显示保存过程
- 改进错误处理机制，区分表单验证错误和网络错误
- 显示详细的错误信息，包括后端返回的错误信息

## 总结

Prerender Shield 已经实现了一个端口一个站点的设计，支持两种访问模式和上游代理功能。用户可以根据实际需求选择合适的部署方案，包括：
- 独立部署，直接对外访问
- 通过nginx等反向代理服务器访问
- 启用上游代理，将请求转发到其他服务

这些功能和改进提高了Prerender Shield的灵活性和易用性，使其能够适应不同的部署场景和需求。