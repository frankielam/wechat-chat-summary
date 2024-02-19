# WeChat Chat Summary

WeChat Chat Summary是一个高效的工具，旨在自动提取、分析，并总结微信聊天记录。该工具能够从指定邮箱中检索微信聊天记录，使用ChatGPT API对其进行智能分析，最后将生成的摘要自动发送到企业微信、飞书或钉钉的Webhook URL。

## 特点

- **自动提取**：从指定邮箱自动检索微信聊天记录。
- **智能分析**：利用最新的ChatGPT API对聊天内容进行分析和总结。
- **多平台通知**：支持将聊天摘要发送到多种企业通讯平台。

## 快速开始

### 通过Docker

1. 克隆此仓库：

   ```bash
   git clone https://github.com/frankielam/wechat-chat-summary.git
   ```
2. 修改配置文件 config.json

3. 构建Docker镜像：

    ```bash
    docker build -t wechat-chat-summary:latest .
    ```

4. 运行容器：

    ```bash
    docker run -d --name wechat-chat-summary wechat-chat-summary:latest
    ```

### 直接运行

确保您的环境中安装了Go (版本1.15或更高)。

1. 克隆仓库并进入目录：

    ```bash
    git clone https://github.com/frankielam/wechat-chat-summary.git
    cd wechat-chat-summary
    ```

2. 编译程序：

    ```bash
    go build
    ```
3. 修改配置文件 config.json

4. 运行程序：

    ```bash
    ./wechat-chat-summary
    ```

## 使用示例

（在这里提供如何使用程序的具体示例，包括如何配置和启动程序，以及如何查看处理结果。）

## 配置说明

（详细说明`config.json`配置文件的结构，包括每个字段的含义和如何填写。）
    ```json
    {
        "email": {
        "imapServer": "imap.qq.com:993",
        "username": "<your>@qq.com",        // QQ 邮箱
        "password": "abckmzxbnbvxbabc",     // QQ 邮箱生成的授权码
        "mailbox": "INBOX"
        },
        "webhook": {
        "weComUrl": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=12345678-1234-1234-1234-007123456789"    // 企业微信聊天机器人 webhool
        },
        "llm": {
        "host": "https://dashscope.aliyuncs.com",   // 通义千问 | ChatGPT (https://api.openai.com) 
        "api": "/api/v1/services/aigc/text-generation/generation",  // 通义千问 | ChatGPT (/v1/chat/completions")
        "token": "sk-123fe3d5a6f44c8f8a0472ee509ec123", // 对应 TOKEN
        "model": "qwen-max",                            // 千问 | GPT-4
        "prompt": "请根据以下三个等号分隔符内的微信聊天记录，提供一个简洁的摘要，包括主要讨论的话题和关键信息点。\r\n===\r\n[CHAT-RECORD]\r\n==="                           // [CHAT-RECORD] 是聊天记录占位符
        }
    }
    ```

## 环境变量

（如果支持通过环境变量配置程序，请在这里列出所有支持的环境变量及其用途。）
为了方便Docker部署，本程序支持通过环境变量覆盖config.json中的配置。支持的环境变量包括：

    ```
    IMAP_SERVER: IMAP服务器地址。
    USERNAME: 邮箱用户名。
    PASSWORD: 邮箱密码/授权码。
    WECOM_URL: 企业微信Webhook URL。
    HOST: ChatGPT API HOST, 如 https://api.openai.com。
    API: ChatGPT API endpoint, 如 /v1/chat/completions。
    TOKEN: ChatGPT TOKEN。
    MODEL: ChatGPT 模型, ChatGPT-turbo | GPT-4。
    PROMPT: prompt模板，需含有字符串`[CHAT-RECORD]`, `[CHAT-RECORD]`为聊天记录的占位符。
    ```

## 贡献指南

欢迎任何形式的贡献，包括功能请求、bug报告、代码贡献等。

## 许可证

该项目采用MIT许可证。详情见[LICENSE](LICENSE)文件。

