# Prerender Shield 双仓库同步指南

本文档提供将 Prerender Shield 项目同步到 Gitee 和 GitHub 的详细指南。

## 📋 当前仓库状态

根据检测，您的项目当前配置如下：
- **当前远程仓库**: `origin` -> `git@gitee.com:xhpmayun/prerender-shield.git`
- **当前分支**: `master`
- **GitHub 仓库**: 未配置

## 🚀 快速开始

### 方法一：使用同步脚本（推荐）

1. **首次运行同步脚本**：
   ```bash
   ./sync-repos.sh
   ```

2. **选择配置双仓库推送**（选项 2）

3. **输入 GitHub 仓库 URL**（例如：`git@github.com:yourname/prerender-shield.git`）

4. **完成配置后，使用选项 4 推送代码**

### 方法二：手动配置

#### 步骤 1：添加 GitHub 远程仓库
```bash
# 将 yourname 替换为你的 GitHub 用户名
git remote add github git@github.com:yourname/prerender-shield.git

# 验证远程仓库配置
git remote -v
```

#### 步骤 2：配置双仓库推送（可选但推荐）
```bash
# 配置同时推送到 Gitee 和 GitHub
git remote set-url --add --push origin git@gitee.com:xhpmayun/prerender-shield.git
git remote set-url --add --push origin git@github.com:yourname/prerender-shield.git

# 验证推送配置
git remote show origin
```

#### 步骤 3：推送代码
```bash
# 方法 A：使用双仓库推送（如果配置了步骤 2）
git push

# 方法 B：分别推送
git push origin master    # 推送到 Gitee
git push github master    # 推送到 GitHub
```

## 📖 详细说明

### 1. 同步脚本功能介绍

`sync-repos.sh` 脚本提供完整的双仓库管理功能：

```bash
# 显示交互式菜单
./sync-repos.sh

# 或使用命令行参数
./sync-repos.sh setup    # 配置双仓库同步
./sync-repos.sh pull     # 拉取最新代码
./sync-repos.sh push     # 推送代码到所有仓库
./sync-repos.sh status   # 显示仓库状态
./sync-repos.sh help     # 显示帮助信息
```

#### 脚本功能选项：
1. **检查仓库配置** - 显示当前远程仓库信息
2. **配置双仓库推送** - 设置一键推送到 Gitee 和 GitHub
3. **拉取最新代码** - 从两个仓库拉取更新
4. **推送代码到所有仓库** - 一键推送到所有配置的仓库
5. **手动分别推送** - 分别推送到每个仓库
6. **显示仓库状态** - 查看分支、提交和更改状态
7. **退出** - 退出脚本

### 2. Git 配置详解

#### 双仓库推送配置原理
配置后，`.git/config` 文件会包含类似内容：
```ini
[remote "origin"]
    url = git@gitee.com:xhpmayun/prerender-shield.git
    fetch = +refs/heads/*:refs/remotes/origin/*
    pushurl = git@gitee.com:xhpmayun/prerender-shield.git
    pushurl = git@github.com:yourname/prerender-shield.git
```

这样配置后，执行 `git push` 会自动推送到两个仓库。

#### 查看当前配置
```bash
# 查看所有远程仓库
git remote -v

# 查看详细的推送配置
git config --get-all remote.origin.pushurl

# 或查看完整的 Git 配置
cat .git/config
```

### 3. 同步工作流程

#### 日常开发流程
```bash
# 1. 开发新功能
git checkout -b feature/new-feature
# ... 编写代码 ...

# 2. 提交更改
git add .
git commit -m "添加新功能"

# 3. 推送到两个仓库
git push -u origin feature/new-feature

# 4. 在 Gitee/GitHub 创建 Pull Request
```

#### 同步现有更改
```bash
# 如果已经在本地有提交，需要同步到两个仓库
./sync-repos.sh push

# 或手动操作
git push origin master
git push github master
```

#### 从两个仓库拉取更新
```bash
# 使用脚本拉取
./sync-repos.sh pull

# 或手动拉取
git pull origin master
git pull github master
```

### 4. 解决常见问题

#### 问题 1：GitHub 仓库不存在
**解决方案**：
1. 在 GitHub 上创建同名仓库 `prerender-shield`
2. 确保仓库为空（不要初始化 README、.gitignore 等）
3. 获取仓库 URL（SSH 格式）：`git@github.com:yourname/prerender-shield.git`

#### 问题 2：SSH 密钥配置
**检查 SSH 密钥**：
```bash
# 测试 Gitee 连接
ssh -T git@gitee.com

# 测试 GitHub 连接  
ssh -T git@github.com
```

**如果连接失败**：
1. 生成 SSH 密钥（如果还没有）：
   ```bash
   ssh-keygen -t ed25519 -C "your_email@example.com"
   ```

2. 将公钥添加到 Gitee 和 GitHub：
   - Gitee: https://gitee.com/profile/sshkeys
   - GitHub: https://github.com/settings/keys

#### 问题 3：推送冲突
**解决方案**：
```bash
# 1. 先拉取最新代码
git pull origin master
git pull github master

# 2. 解决冲突
# ... 解决文件冲突 ...

# 3. 重新提交
git add .
git commit -m "解决合并冲突"

# 4. 推送
git push
```

#### 问题 4：只想推送到一个仓库
```bash
# 只推送到 Gitee
git push origin master

# 只推送到 GitHub
git push github master
```

### 5. 自动化脚本

#### 创建 Git Hook 自动同步
在 `.git/hooks/post-commit` 中添加：
```bash
#!/bin/bash
# 自动推送到两个仓库
git push origin master
git push github master
```

设置执行权限：
```bash
chmod +x .git/hooks/post-commit
```

#### 使用 CI/CD 自动同步
在 GitHub Actions 或 Gitee Go 中配置工作流，实现自动双向同步。

## 🔄 同步策略建议

### 策略一：主从模式（推荐）
- **主仓库**: Gitee（作为主要开发仓库）
- **从仓库**: GitHub（作为镜像仓库）
- **工作流程**: 所有开发在 Gitee 进行，自动同步到 GitHub

### 策略二：双向同步模式
- **两个仓库平等**
- **工作流程**: 可以从任意仓库拉取和推送
- **注意事项**: 需要确保两个仓库内容一致，避免冲突

### 策略三：分支对应模式
```bash
# 为不同仓库创建不同分支
git checkout -b github-main
git push github github-main:main

# 或保持分支名称一致
git push origin master
git push github master
```

## 📊 仓库维护

### 定期检查
```bash
# 检查仓库状态
./sync-repos.sh status

# 检查两个仓库的差异
git fetch --all
git log --oneline origin/master..github/master
git log --oneline github/master..origin/master
```

### 清理和优化
```bash
# 清理无效的远程分支引用
git remote prune origin
git remote prune github

# 优化本地仓库
git gc --auto
```

## 🆘 故障排除

### 错误：远程仓库已存在
```bash
# 删除现有的 GitHub 远程仓库
git remote remove github

# 重新添加
git remote add github git@github.com:yourname/prerender-shield.git
```

### 错误：认证失败
```bash
# 检查 SSH 配置
ssh -vT git@github.com

# 切换为 HTTPS（如果需要）
git remote set-url github https://github.com/yourname/prerender-shield.git
```

### 错误：分支不匹配
```bash
# 如果 GitHub 使用 main 分支
git push github master:main

# 或重命名本地分支
git branch -m master main
git push -u origin main
git push -u github main
```

## 📝 最佳实践

1. **保持提交历史一致**：在两个仓库保持相同的提交历史
2. **定期同步**：每天至少同步一次，避免大量冲突
3. **使用有意义的提交信息**：便于跟踪更改
4. **测试同步**：在重要更改前测试同步流程
5. **备份配置**：备份 `.git/config` 文件

## 🔗 相关资源

- [Gitee 帮助中心](https://gitee.com/help)
- [GitHub 文档](https://docs.github.com/cn)
- [Git 官方文档](https://git-scm.com/doc)
- [SSH 密钥生成指南](https://docs.github.com/cn/authentication/connecting-to-github-with-ssh)

## 📞 支持

如果遇到问题：
1. 查看本指南的故障排除部分
2. 检查脚本错误信息
3. 查看项目文档
4. 在 GitHub/Gitee 仓库提交 Issue

---

**最后更新**: 2026-01-07  
**维护者**: Prerender Shield 项目组  
**文档版本**: v1.0