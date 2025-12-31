<script setup>
const scrollToSection = (id) => {
  const element = document.getElementById(id);
  if (element) {
    element.scrollIntoView({
      behavior: 'smooth',
      block: 'start'
    });
  }
};
</script>

<template>
  <div id="app">
    <!-- 导航栏 -->
    <nav class="nav">
      <div class="nav-container">
        <div class="nav-logo">
          <h1 class="glow">PreRender Shield</h1>
        </div>
        <div class="nav-links">
          <a href="#home" class="nav-link">首页</a>
          <a href="#principle" class="nav-link">产品原理</a>
          <a href="#docs" class="nav-link">文档</a>
          <a href="#contact" class="nav-link">联系我们</a>
        </div>
      </div>
    </nav>

    <!-- 英雄区域 -->
    <section id="home" class="hero">
      <div class="hero-content">
        <h1 class="hero-title glow">PreRender Shield</h1>
        <p class="hero-subtitle">下一代 Web 应用防火墙，守护您的数字资产安全</p>
        <div class="hero-buttons">
          <button @click="scrollToSection('principle')">了解原理</button>
          <button @click="scrollToSection('docs')">查看文档</button>
        </div>
      </div>
    </section>

    <!-- 产品原理 -->
    <section id="principle" class="section">
      <h2>产品原理</h2>
      <div class="container">
        <div class="card">
          <h3>智能语义分析</h3>
          <p>基于先进的机器学习算法，深度理解 Web 流量的语义结构，精准识别各种攻击模式，包括 0day 漏洞攻击。</p>
        </div>
        <div class="card">
          <h3>无规则引擎设计</h3>
          <p>突破传统 WAF 依赖规则库的限制，采用线性安全检测算法，平均检测延迟 < 1 毫秒，单核轻松处理 2000+ TPS 并发。</p>
        </div>
        <div class="card">
          <h3>多维防护体系</h3>
          <p>融合人机验证、恶意 IP 情报、CC 攻击防护等多种防护机制，构建全方位的 Web 安全防护体系。</p>
        </div>
        <div class="card">
          <h3>高性能架构</h3>
          <p>基于 Nginx 开发，拥有完善的健康检查机制，确保高可用性和稳定性，为您的业务提供持续可靠的安全保障。</p>
        </div>
      </div>
    </section>

    <!-- 文档部分 -->
    <section id="docs" class="section">
      <h2>文档中心</h2>
      
      <!-- 安装文档 -->
      <div class="card">
        <h3>安装文档</h3>
        <div class="install-steps">
          <div class="step">
            <h4>步骤 1：准备环境</h4>
            <p>确保您的服务器满足以下要求：</p>
            <ul>
              <li>Linux 操作系统（推荐 Ubuntu 20.04+ 或 CentOS 7+）</li>
              <li>至少 2GB 内存</li>
              <li>至少 20GB 磁盘空间</li>
              <li>Docker 环境（推荐 Docker 20.10+）</li>
            </ul>
          </div>
          <div class="step">
            <h4>步骤 2：下载安装包</h4>
            <pre><code>wget https://example.com/prerender-shield-latest.tar.gz</code></pre>
          </div>
          <div class="step">
            <h4>步骤 3：解压安装包</h4>
            <pre><code>tar -zxvf prerender-shield-latest.tar.gz</code></pre>
          </div>
          <div class="step">
            <h4>步骤 4：执行安装脚本</h4>
            <pre><code>cd prerender-shield && ./install.sh</code></pre>
          </div>
          <div class="step">
            <h4>步骤 5：访问管理界面</h4>
            <p>安装完成后，通过浏览器访问：<a href="http://your-server-ip:8000" target="_blank">http://your-server-ip:8000</a></p>
          </div>
        </div>
      </div>

      <!-- 使用文档 -->
      <div class="card">
        <h3>使用文档</h3>
        <div class="usage-docs">
          <div class="doc-section">
            <h4>快速开始</h4>
            <p>安装完成后，您可以按照以下步骤快速配置防护规则：</p>
            <ol>
              <li>登录管理界面</li>
              <li>在「站点管理」中添加您的网站</li>
              <li>配置域名和后端服务器地址</li>
              <li>启用所需的防护模块</li>
              <li>保存配置并应用</li>
            </ol>
          </div>
          <div class="doc-section">
            <h4>防护模块说明</h4>
            <ul>
              <li><strong>SQL 注入防护</strong>：检测并拦截各种 SQL 注入攻击</li>
              <li><strong>XSS 防护</strong>：防止跨站脚本攻击</li>
              <li><strong>CC 攻击防护</strong>：有效抵御分布式拒绝服务攻击</li>
              <li><strong>文件上传防护</strong>：检测并阻止恶意文件上传</li>
              <li><strong>API 防护</strong>：保护 RESTful API 接口安全</li>
            </ul>
          </div>
          <div class="doc-section">
            <h4>监控与日志</h4>
            <p>PreRender Shield 提供了详细的监控和日志功能：</p>
            <ul>
              <li>实时流量监控</li>
              <li>攻击事件日志</li>
              <li>性能统计报告</li>
              <li>告警通知配置</li>
            </ul>
          </div>
        </div>
      </div>
    </section>

    <!-- 联系我们 -->
    <section id="contact" class="section">
      <h2>联系我们</h2>
      <div class="contact-container">
        <div class="contact-info">
          <div class="contact-item">
            <h3>微信</h3>
            <p>xiao142000</p>
          </div>
          <div class="contact-item">
            <h3>QQ</h3>
            <p>1036698712</p>
          </div>
          <div class="contact-item">
            <h3>邮箱</h3>
            <p>myloveisphp@126.com</p>
          </div>
        </div>
        <div class="contact-form">
          <h3>发送消息</h3>
          <form>
            <div class="form-group">
              <label for="name">姓名</label>
              <input type="text" id="name" name="name" required>
            </div>
            <div class="form-group">
              <label for="email">邮箱</label>
              <input type="email" id="email" name="email" required>
            </div>
            <div class="form-group">
              <label for="message">留言</label>
              <textarea id="message" name="message" rows="5" required></textarea>
            </div>
            <button type="submit">发送</button>
          </form>
        </div>
      </div>
    </section>

    <!-- 页脚 -->
    <footer class="footer">
      <div class="footer-content">
        <p>&copy; 2025 PreRender Shield. All rights reserved.</p>
      </div>
    </footer>
  </div>
</template>

<style scoped>
/* 导航栏样式 */
.nav {
  background: rgba(10, 10, 10, 0.9);
  border-bottom: 1px solid rgba(0, 255, 255, 0.2);
  padding: 1rem 0;
  position: sticky;
  top: 0;
  z-index: 100;
  backdrop-filter: blur(10px);
}

.nav-container {
  display: flex;
  justify-content: space-between;
  align-items: center;
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 2rem;
}

.nav-logo h1 {
  margin: 0;
  font-size: 1.8em;
  cursor: pointer;
}

.nav-links {
  display: flex;
  gap: 2rem;
}

.nav-link {
  font-size: 1.1em;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 1px;
}

/* 英雄区域样式 */
.hero {
  min-height: 80vh;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 4rem 0;
  position: relative;
}

.hero-content {
  max-width: 800px;
}

.hero-title {
  font-size: 4.5em;
  margin-bottom: 1rem;
  line-height: 1.1;
}

.hero-subtitle {
  font-size: 1.5em;
  margin-bottom: 2.5rem;
  color: rgba(255, 255, 255, 0.8);
}

.hero-buttons {
  display: flex;
  gap: 1.5rem;
  justify-content: center;
  flex-wrap: wrap;
}

/* 步骤样式 */
.install-steps, .usage-docs {
  margin-top: 2rem;
}

.step, .doc-section {
  margin-bottom: 2rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid rgba(0, 255, 255, 0.1);
}

.step:last-child, .doc-section:last-child {
  border-bottom: none;
}

.step h4, .doc-section h4 {
  color: #00ffff;
  margin-bottom: 1rem;
  font-size: 1.3em;
}

pre {
  background: rgba(0, 0, 0, 0.5);
  padding: 1rem;
  border-radius: 8px;
  overflow-x: auto;
  border: 1px solid rgba(0, 255, 255, 0.3);
}

code {
  font-family: 'Courier New', Courier, monospace;
  color: #00ff00;
  font-size: 1em;
}

ol, ul {
  padding-left: 2rem;
  margin: 1rem 0;
  color: rgba(255, 255, 255, 0.85);
}

li {
  margin: 0.5rem 0;
}

/* 联系我们样式 */
.contact-container {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 2rem;
  margin-top: 2rem;
}

.contact-info {
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.contact-item h3 {
  margin-bottom: 0.5rem;
  color: #00ffff;
}

.contact-item p {
  font-size: 1.2em;
  margin: 0;
}

.contact-form form {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.form-group label {
  color: #00ffff;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 1px;
  font-size: 0.9em;
}

.form-group input,
.form-group textarea {
  padding: 0.8rem;
  border: 1px solid rgba(0, 255, 255, 0.3);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.3);
  color: white;
  font-size: 1em;
  font-family: inherit;
  transition: all 0.3s ease;
}

.form-group input:focus,
.form-group textarea:focus {
  outline: none;
  border-color: #00ffff;
  box-shadow: 0 0 10px rgba(0, 255, 255, 0.3);
}

.form-group input::placeholder,
.form-group textarea::placeholder {
  color: rgba(255, 255, 255, 0.5);
}

/* 页脚样式 */
.footer {
  background: rgba(10, 10, 10, 0.9);
  border-top: 1px solid rgba(0, 255, 255, 0.2);
  padding: 2rem 0;
  margin-top: 4rem;
  text-align: center;
}

.footer-content p {
  margin: 0;
  color: rgba(255, 255, 255, 0.6);
  font-size: 1em;
}

/* 滚动动画 */
@keyframes fadeInUp {
  from {
    opacity: 0;
    transform: translateY(30px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.card {
  animation: fadeInUp 0.8s ease-out;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .nav-container {
    flex-direction: column;
    gap: 1rem;
    padding: 0 1rem;
  }

  .nav-links {
    gap: 1rem;
    flex-wrap: wrap;
    justify-content: center;
  }

  .hero-title {
    font-size: 3em;
  }

  .hero-subtitle {
    font-size: 1.2em;
  }

  .hero-buttons {
    flex-direction: column;
    align-items: center;
  }

  .contact-container {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .hero-title {
    font-size: 2.5em;
  }

  .nav-link {
    font-size: 1em;
  }
}
</style>
