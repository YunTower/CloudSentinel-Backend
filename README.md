# CloudSentinel Backend

CloudSentinel 面板后端，负责 API、WebSocket、数据存储、告警、更新和 Agent 通信。

完整安装入口请看主仓库：[YunTower/CloudSentinel](https://github.com/YunTower/CloudSentinel)。

## 一键安装

```bash
curl -L https://raw.githubusercontent.com/YunTower/CloudSentinel-Scripts/refs/heads/master/backend/install.sh -o cloudsentinel.sh && chmod +x cloudsentinel.sh && sudo ./cloudsentinel.sh
```

安装完成后会输出管理端地址、公开端地址、管理员账号和密码。Backend 在同一个进程中监听两个端口：管理端口提供完整 API、Agent 通信和 Admin SPA，公开端口只提供 Public SPA 与四个只读公开接口。请同时放行脚本输出的两个端口。

手动配置时使用：

```env
APP_HOST=0.0.0.0
APP_PORT=3000
PUBLIC_HTTP_ENABLED=true
PUBLIC_HOST=0.0.0.0
PUBLIC_PORT=3001
```

## 常用命令

安装脚本会创建 `cloudsentinel` 全局命令：

```bash
cloudsentinel start
cloudsentinel stop
cloudsentinel restart
cloudsentinel panel:info
cloudsentinel update
cloudsentinel uninstall
```

如全局命令不可用，可进入安装目录执行 `./dashboard <command>`。

## 开发

```bash
go run . migrate
go run . generate:admin
go run . start
```

## 相关仓库

- 主仓库/前端：[YunTower/CloudSentinel](https://github.com/YunTower/CloudSentinel)
- Agent：[YunTower/CloudSentinel-Agent](https://github.com/YunTower/CloudSentinel-Agent)

## 部署限制与安全说明

- **单实例部署**：服务监测的多探测点轮次聚合状态保存在面板进程内存中，仅支持单实例部署。多实例会导致各实例只能收集到自己下发的探测结果、状态互相覆盖。
- **JWT 吊销黑名单**：登出/改密后令牌写入缓存黑名单。默认缓存驱动（内存/文件）下面板重启会清空黑名单，已"登出"的令牌在剩余有效期内会复活；需要跨重启吊销时请配置持久化缓存驱动（如 Redis）。
- **密钥管理**：`.env` 中的 `APP_KEY` 用于加密告警通知配置等敏感字段，`JWT_SECRET` 用于签发管理员会话；请勿随仓库或镜像分发，泄露后应在安全环境轮换（轮换 `APP_KEY` 会使已加密的配置失效，需重新保存）。
- **Agent 命令签名**：下发到 Agent 的 service_check / restart / update_config / update 命令均附带面板 RSA 签名（密钥对存于 `system_settings` 的 `panel_rsa_keys`）；Agent 会验签后才执行，敏感命令还要求已建立加密会话。
- **内网探测开关**：服务监测默认禁止探测内网/保留地址（防 SSRF），如需监测内网目标，请在系统设置中显式开启 `monitor_allow_private_targets`。
