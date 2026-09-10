<template>
  <AppLayout>
    <div class="docs-page">
      <aside class="docs-toc" aria-label="文档目录">
        <a
          v-for="item in tocItems"
          :key="item.id"
          :href="`#${item.id}`"
          class="docs-toc-link"
        >
          <span>{{ item.kicker }}</span>
          {{ item.label }}
        </a>
      </aside>

      <main class="docs-main">
        <section id="overview" class="docs-hero">
          <div>
            <p class="docs-eyebrow">TogoAPI Docs</p>
            <h1>TogoAPI 用户使用文档</h1>
            <p class="docs-lead">
              从注册、创建 API Key，到 CCSwitch、Claude Code、Codex 和第三方 OpenAI 兼容客户端配置，
              这份教程按新手实际路径整理，照着做即可完成接入。
            </p>
            <div class="docs-actions">
              <a href="#quick-start" class="docs-button primary">开始配置</a>
              <RouterLink to="/models" class="docs-button">查看模型广场</RouterLink>
              <RouterLink to="/keys" class="docs-button">创建 Key</RouterLink>
            </div>
          </div>
          <div class="docs-endpoint-card">
            <span>API Base URL</span>
            <code>{{ apiBaseUrl }}</code>
            <button type="button" @click="copyCode(apiBaseUrl, 'base-url')">
              {{ copiedKey === 'base-url' ? '已复制' : '复制' }}
            </button>
          </div>
        </section>

        <section class="docs-path-grid" aria-label="文档核心路径">
          <article v-for="card in pathCards" :key="card.title" class="docs-card">
            <span>{{ card.step }}</span>
            <h2>{{ card.title }}</h2>
            <p>{{ card.text }}</p>
            <a :href="card.href">{{ card.link }}</a>
          </article>
        </section>

        <section id="quick-start" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">Quick Start</p>
            <h2>快速开始</h2>
            <p>第一次使用建议按顺序完成；已有账号时可以直接跳到创建 Key 和客户端配置。</p>
          </div>
          <div class="docs-step-list">
            <div v-for="step in quickSteps" :key="step" class="docs-step">
              <span>{{ quickSteps.indexOf(step) + 1 }}</span>
              <p>{{ step }}</p>
            </div>
          </div>
          <CodeBlock
            label="客户端通用填写"
            :code="baseUrlAndApiKey"
            copy-id="quick-client"
            :copied-key="copiedKey"
            @copy="copyCode"
          />
        </section>

        <section id="account" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">Account</p>
            <h2>账号与 Key</h2>
            <p>后台入口、注册、创建 Key 和安全注意事项都集中在这里。</p>
          </div>
          <div class="docs-split">
            <article class="docs-panel">
              <h3>注册账号</h3>
              <ol>
                <li>打开 <a :href="apiBaseUrl">{{ apiBaseUrl }}</a>。</li>
                <li>点击注册或登录入口。</li>
                <li>使用邮箱或用户名创建账号，并按页面提示完成验证。</li>
                <li>注册完成后进入用户后台。</li>
              </ol>
              <details>
                <summary>收不到验证邮件</summary>
                <ol>
                  <li>检查垃圾邮件、广告邮件、订阅邮件文件夹。</li>
                  <li>确认邮箱地址没有输错。</li>
                  <li>等待 1-3 分钟后重新发送。</li>
                  <li>仍无法收到时，联系站点管理员或页面中展示的客服入口。</li>
                </ol>
              </details>
            </article>
            <ScreenshotCard src="/docs-assets/register-entry.png" alt="TogoAPI 注册入口截图" />
          </div>
          <div class="docs-split reverse">
            <article class="docs-panel">
              <h3>创建专属 Key</h3>
              <ol>
                <li>进入 <RouterLink to="/keys">{{ keysUrl }}</RouterLink>。</li>
                <li>点击“创建密钥”。</li>
                <li>填写 Key 名称，例如 <code>我的电脑</code>、<code>Claude Code</code>、<code>Codex</code>。</li>
                <li>按个人需求选择可用分组，不确定时保持默认。</li>
                <li>创建成功后立即复制并妥善保存。</li>
              </ol>
              <div class="docs-warning">
                <strong>Key 安全提醒</strong>
                <p>不要公开完整 Key。反馈问题时只保留前缀和后几位；怀疑泄露时立即删除旧 Key 并创建新 Key。</p>
              </div>
            </article>
            <div class="docs-image-stack">
              <ScreenshotCard src="/docs-assets/create-key-button.png" alt="创建 Key 按钮截图" />
              <ScreenshotCard src="/docs-assets/create-key-modal.png" alt="创建 Key 表单截图" />
            </div>
          </div>
          <div class="docs-table-wrap">
            <table>
              <thead>
                <tr><th>后台功能</th><th>说明</th></tr>
              </thead>
              <tbody>
                <tr><td>个人资料</td><td>查看账号、余额和基础信息</td></tr>
                <tr><td>API 密钥</td><td>创建、复制、禁用、删除 API Key</td></tr>
                <tr><td>使用记录</td><td>查看请求量、消耗、模型调用和耗时</td></tr>
                <tr><td>模型广场</td><td>查看当前可用模型和价格</td></tr>
                <tr><td>站点检测</td><td>查看模型和渠道可用性</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section id="environment" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">Environment</p>
            <h2>基础环境安装</h2>
            <p>先准备 Node.js 和 Git，再安装 Codex、Claude Code 或 CCSwitch。</p>
          </div>
          <div class="docs-two-col">
            <article class="docs-panel">
              <div class="docs-card-head">
                <h3>Node.js</h3>
                <TabButtons group="node" :tabs="['Windows', 'macOS', 'Linux']" :active-tabs="activeTabs" @select="setTab" />
              </div>
              <div v-if="activeTabs.node === 'Windows'" class="docs-tab-panel">
                <ol>
                  <li>打开 Node.js 官网：<a href="https://nodejs.org/zh-cn/download" target="_blank" rel="noopener noreferrer">https://nodejs.org/zh-cn/download</a>。</li>
                  <li>下载 LTS 长期支持版本的 Windows Installer <code>.msi</code>。</li>
                  <li>运行安装包，采用默认配置安装。</li>
                </ol>
                <ScreenshotCard src="/docs-assets/node-download.png" alt="Node.js 下载页面截图" />
              </div>
              <div v-else-if="activeTabs.node === 'macOS'" class="docs-tab-panel">
                <ol>
                  <li>打开 Node.js 官网，选择 LTS 长期支持版本。</li>
                  <li>Apple Silicon 下载 macOS Apple Silicon 安装包，Intel Mac 下载 macOS Intel 安装包。</li>
                  <li>打开 <code>.pkg</code> 安装包并按提示安装。</li>
                </ol>
                <CodeBlock label="验证 Node.js" code="node --version" copy-id="node-version" :copied-key="copiedKey" @copy="copyCode" />
                <CodeBlock label="验证 npm" code="npm --version" copy-id="npm-version" :copied-key="copiedKey" @copy="copyCode" />
                <CodeBlock label="Homebrew 安装 Node.js" code="brew install node" copy-id="brew-node" :copied-key="copiedKey" @copy="copyCode" />
              </div>
              <div v-else class="docs-tab-panel">
                <p>Linux 用户优先使用发行版包管理器或 NodeSource。服务器环境建议安装 LTS 版本。</p>
                <CodeBlock label="Ubuntu / Debian" code="sudo apt update&#10;sudo apt install -y nodejs npm" copy-id="linux-node-apt" :copied-key="copiedKey" @copy="copyCode" />
                <CodeBlock label="验证版本" code="node -v&#10;npm -v" copy-id="linux-node-version" :copied-key="copiedKey" @copy="copyCode" />
              </div>
              <CodeBlock label="通用验证" code="node -v&#10;npm -v" copy-id="node-npm" :copied-key="copiedKey" @copy="copyCode" />
            </article>

            <article class="docs-panel">
              <div class="docs-card-head">
                <h3>Git</h3>
                <TabButtons group="git" :tabs="['Windows', 'macOS', 'Linux']" :active-tabs="activeTabs" @select="setTab" />
              </div>
              <div v-if="activeTabs.git === 'Windows'" class="docs-tab-panel">
                <p>Windows 用户建议安装 Git for Windows。</p>
                <p>下载地址：<a href="https://git-scm.com/install/windows" target="_blank" rel="noopener noreferrer">https://git-scm.com/install/windows</a></p>
              </div>
              <div v-else-if="activeTabs.git === 'macOS'" class="docs-tab-panel">
                <p>macOS 通常自带或可通过系统开发者工具获得 Git。已安装 Homebrew 的用户可以直接安装或更新。</p>
                <CodeBlock label="Homebrew 安装 Git" code="brew install git" copy-id="brew-git" :copied-key="copiedKey" @copy="copyCode" />
              </div>
              <div v-else class="docs-tab-panel">
                <p>Linux 使用系统包管理器安装 Git。</p>
                <CodeBlock label="Ubuntu / Debian" code="sudo apt update&#10;sudo apt install -y git" copy-id="linux-git" :copied-key="copiedKey" @copy="copyCode" />
              </div>
              <CodeBlock label="验证 Git" code="git --version" copy-id="git-version" :copied-key="copiedKey" @copy="copyCode" />
            </article>
          </div>
        </section>

        <section id="tool-install" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">Tools</p>
            <h2>工具安装</h2>
            <p>安装路径按“工具 -> 使用形态 -> 系统”组织，终端命令都拆成单条可复制。</p>
          </div>
          <TabButtons group="tool" :tabs="['Codex', 'Claude Code']" :active-tabs="activeTabs" @select="setTab" />

          <div v-if="activeTabs.tool === 'Codex'" class="docs-tool-panel">
            <div class="docs-card-head">
              <div>
                <h3>Codex</h3>
                <p>OpenAI 官方代码代理工具，支持 App、CLI 和 IDE 扩展。</p>
              </div>
              <TabButtons group="codexMode" :tabs="['App', 'CLI', 'IDE']" :active-tabs="activeTabs" @select="setTab" compact />
            </div>
            <CodexInstall
              :mode="activeTabs.codexMode"
              :platform="activeTabs.codexPlatform"
              :copied-key="copiedKey"
              @set-platform="setTab('codexPlatform', $event)"
              @copy="copyCode"
            />
          </div>

          <div v-else class="docs-tool-panel">
            <div class="docs-card-head">
              <div>
                <h3>Claude Code</h3>
                <p>Anthropic 官方代码代理工具，CLI 和 IDE 集成是主要使用方式。</p>
              </div>
              <TabButtons group="claudeMode" :tabs="['App', 'CLI', 'IDE']" :active-tabs="activeTabs" @select="setTab" compact />
            </div>
            <ClaudeInstall
              :mode="activeTabs.claudeMode"
              :platform="activeTabs.claudePlatform"
              :copied-key="copiedKey"
              @set-platform="setTab('claudePlatform', $event)"
              @copy="copyCode"
            />
          </div>
        </section>

        <section id="api-config" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">Configuration</p>
            <h2>快速配置指南</h2>
            <p>上一章完成工具安装；本章处理让工具连接 TogoAPI API。</p>
          </div>
          <div class="docs-flow">
            <div><span>1</span>后台创建 API Key</div>
            <div><span>2</span>安装 Codex / Claude Code / Gemini CLI</div>
            <div><span>3</span>Claude Code 新用户可先运行 ZCF</div>
            <div><span>4</span>用 CCSwitch 统一管理 API 配置</div>
          </div>
          <CodeBlock
            label="TogoAPI API 连接信息"
            :code="baseUrlAndApiKey"
            copy-id="jqcode-connect"
            :copied-key="copiedKey"
            @copy="copyCode"
          />
          <article class="docs-panel">
            <h3>Claude Code 初始化：ZCF</h3>
            <p>ZCF 偏向 Claude Code 初始化，适合第一次配置 Claude Code，或需要重新整理 Claude Code 环境的用户。</p>
            <p>项目地址：<a href="https://github.com/UfoMiao/zcf" target="_blank" rel="noopener noreferrer">https://github.com/UfoMiao/zcf</a></p>
            <CodeBlock label="运行 ZCF" code="npx zcf" copy-id="npx-zcf" :copied-key="copiedKey" @copy="copyCode" />
            <ul>
              <li>只使用 Claude Code 时，可以先运行 ZCF，再用 CCSwitch 管理 API 供应商。</li>
              <li>同时使用 Codex、Claude Code、Gemini CLI 时，建议把 API 配置统一交给 CCSwitch。</li>
              <li>ZCF 不等于 TogoAPI API Key，它主要负责初始化 Claude Code 环境。</li>
            </ul>
          </article>
        </section>

        <section id="ccswitch" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">CCSwitch</p>
            <h2>CCSwitch 使用</h2>
            <p>CCSwitch 适合统一管理 Claude Code、Codex、Gemini CLI 等工具的 API 供应商配置。</p>
          </div>
          <div class="docs-split">
            <article class="docs-panel">
              <p>项目地址：<a href="https://github.com/farion1231/cc-switch" target="_blank" rel="noopener noreferrer">https://github.com/farion1231/cc-switch</a></p>
              <div class="docs-feature-grid">
                <span>供应商配置切换</span>
                <span>端点速度测试</span>
                <span>系统提示预设</span>
                <span>MCP 管理</span>
                <span>Claude Skills</span>
                <span>配置备份恢复</span>
                <span>深度链接协议</span>
                <span>环境变量检测</span>
              </div>
            </article>
            <ScreenshotCard src="/docs-assets/ccswitch-icon.png" alt="CCSwitch 桌面图标" compact />
          </div>
          <ScreenshotCard src="/docs-assets/ccswitch-overview.png" alt="CCSwitch 界面示例" />
          <div class="docs-gallery">
            <ScreenshotCard src="/docs-assets/ccswitch-download.png" alt="CCSwitch 下载截图" caption="下载最新安装器" />
            <ScreenshotCard src="/docs-assets/ccswitch-provider-button.png" alt="CCSwitch 添加供应商按钮" caption="添加供应商入口" />
            <ScreenshotCard src="/docs-assets/ccswitch-provider-config.png" alt="CCSwitch 添加供应商配置" caption="填写 Base URL 和 API Key" />
            <ScreenshotCard src="/docs-assets/ccswitch-claude-code.png" alt="Claude Code 配置示例" caption="Claude Code 示例" />
            <ScreenshotCard src="/docs-assets/ccswitch-codex.png" alt="Codex 配置示例" caption="Codex 示例" />
            <ScreenshotCard src="/docs-assets/ccswitch-routing.png" alt="CCSwitch 路由功能" caption="路由功能" />
          </div>
          <div class="docs-table-wrap">
            <table>
              <thead><tr><th>工具</th><th>建议配置方式</th></tr></thead>
              <tbody>
                <tr><td>Claude Code CLI</td><td>先用 ZCF 初始化，再用 CCSwitch 管理 API 供应商</td></tr>
                <tr><td>Codex CLI</td><td>安装 CLI 后，用 CCSwitch 或工具自身配置入口填写 TogoAPI API</td></tr>
                <tr><td>Gemini CLI</td><td>安装 CLI 后，用 CCSwitch 统一管理 API 配置</td></tr>
                <tr><td>Codex / Claude Code App</td><td>按同样的 Base URL 和 API Key 填写</td></tr>
                <tr><td>IDE 扩展</td><td>先安装扩展，再按扩展提示登录或连接对应 CLI；需要 API 时使用 CCSwitch 中的供应商配置</td></tr>
              </tbody>
            </table>
          </div>
        </section>

        <section id="cli-config" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">CLI</p>
            <h2>CLI 配置教程</h2>
            <p>先选择要配置的工具，再选择系统。Claude Code、Codex 和第三方通用配置分开展示，避免抄错文件。</p>
          </div>
          <div class="docs-config-tabs">
            <div>
              <p class="docs-mini-label">选择工具</p>
              <TabButtons group="cliTool" :tabs="['Claude Code', 'Codex', '第三方通用']" :active-tabs="activeTabs" @select="setTab" />
            </div>
            <div v-if="activeTabs.cliTool !== '第三方通用'">
              <p class="docs-mini-label">选择系统</p>
              <TabButtons group="cliPlatform" :tabs="['Windows', 'macOS', 'Linux']" :active-tabs="activeTabs" @select="setTab" />
            </div>
          </div>

          <article v-if="activeTabs.cliTool === 'Claude Code'" class="docs-panel docs-config-panel">
            <div class="docs-card-head">
              <div>
                <h3>Claude Code 配置</h3>
                <p>Claude Code 只需要关注 <code>settings.json</code>。这里不要复制 Codex 的 <code>config.toml</code> 或 <code>auth.json</code>。</p>
              </div>
              <span class="docs-tool-badge">Claude Code</span>
            </div>
            <CodeBlock
              label="Claude Code 文件路径：settings.json"
              :code="cliPaths[cliPlatform].settings"
              copy-id="claude-settings-path"
              :copied-key="copiedKey"
              @copy="copyCode"
            />
            <CodeBlock
              label="Claude Code 配置内容：settings.json"
              :code="settingsJson"
              copy-id="claude-settings-json"
              :copied-key="copiedKey"
              @copy="copyCode"
            />
          </article>

          <article v-else-if="activeTabs.cliTool === 'Codex'" class="docs-panel docs-config-panel">
            <div class="docs-card-head">
              <div>
                <h3>Codex 配置</h3>
                <p>Codex 主要使用 <code>config.toml</code> 选择供应商和模型，<code>auth.json</code> 保存 OpenAI 兼容 Key。</p>
              </div>
              <span class="docs-tool-badge">Codex</span>
            </div>
            <div class="docs-two-col">
              <CodeBlock
                label="Codex 文件路径：config.toml"
                :code="cliPaths[cliPlatform].config"
                copy-id="codex-config-path"
                :copied-key="copiedKey"
                @copy="copyCode"
              />
              <CodeBlock
                label="Codex 文件路径：auth.json"
                :code="cliPaths[cliPlatform].auth"
                copy-id="codex-auth-path"
                :copied-key="copiedKey"
                @copy="copyCode"
              />
              <CodeBlock
                label="Codex 配置内容：config.toml"
                :code="configToml"
                copy-id="codex-config-toml"
                :copied-key="copiedKey"
                @copy="copyCode"
              />
              <CodeBlock
                label="Codex 配置内容：auth.json"
                :code="authJson"
                copy-id="codex-auth-json"
                :copied-key="copiedKey"
                @copy="copyCode"
              />
            </div>
          </article>

          <article v-else class="docs-panel docs-config-panel">
            <div class="docs-card-head">
              <div>
                <h3>第三方通用配置</h3>
                <p>用于 Cherry Studio、Apifox、Postman、OpenAI 兼容 SDK 等工具。它不是 Claude Code 或 Codex 的本地配置文件。</p>
              </div>
              <span class="docs-tool-badge">OpenAI Compatible</span>
            </div>
            <div class="docs-two-col">
              <CodeBlock
                label="Base URL 和 API Key"
                :code="baseUrlAndApiKey"
                copy-id="third-party-base-key"
                :copied-key="copiedKey"
                @copy="copyCode"
              />
              <CodeBlock
                label="第三方通用请求头"
                code="Authorization: Bearer sk-xxxxxx"
                copy-id="third-party-auth-header"
                :copied-key="copiedKey"
                @copy="copyCode"
              />
            </div>
          </article>
        </section>

        <section id="third-party" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">OpenAI Compatible</p>
            <h2>第三方接入</h2>
            <p>支持 OpenAI 兼容接口或自定义 API 供应商的工具，一般填写 Base URL、API Key 和模型名即可。</p>
          </div>
          <div class="docs-two-col">
            <article class="docs-panel">
              <h3>OpenAI 兼容填写</h3>
              <ol>
                <li>Base URL 填写 <code>{{ apiBaseUrl }}</code>。</li>
                <li>API Key 填写后台创建的 Key，例如 <code>sk-xxxxxx</code>。</li>
                <li>模型名称以模型广场或后台展示为准。</li>
                <li>如果工具要求完整路径，按接口类型选择 <code>/v1/chat/completions</code>、<code>/responses</code> 或 <code>/v1/messages</code>。</li>
              </ol>
            </article>
            <CodeBlock
              label="curl 测试"
              :code="curlExample"
              copy-id="curl-example"
              :copied-key="copiedKey"
              @copy="copyCode"
            />
          </div>
        </section>

        <section id="recommendations" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">Recommended</p>
            <h2>工具推荐</h2>
            <p>这些是围绕 Codex / Claude Code 使用体验的补充工具。</p>
          </div>
          <div class="docs-two-col">
            <article class="docs-panel">
              <div class="docs-card-head">
                <div>
                  <h3>AI Session Viewer</h3>
                  <p>本地会话浏览器，用于浏览、搜索、统计 Claude Code 和 Codex CLI 的本地会话记录，并支持一键 Resume。</p>
                </div>
                <TabButtons group="sessionViewer" :tabs="['Windows', 'macOS', 'Linux']" :active-tabs="activeTabs" @select="setTab" compact />
              </div>
              <div class="docs-link-grid">
                <a href="https://github.com/zuoliangyu/AI-Session-Viewer" target="_blank" rel="noopener noreferrer">GitHub 项目</a>
                <a href="https://github.com/zuoliangyu/AI-Session-Viewer/releases" target="_blank" rel="noopener noreferrer">Releases 下载页</a>
              </div>
              <ScreenshotCard src="/docs-assets/session-viewer.png" alt="AI Session Viewer 界面截图" />
              <p v-if="activeTabs.sessionViewer === 'Windows'">下载 <code>.msi</code> 安装版或 <code>.zip</code> 便携版。</p>
              <p v-else-if="activeTabs.sessionViewer === 'macOS'">下载 Universal <code>.dmg</code>，同时支持 Intel 和 Apple Silicon。</p>
              <p v-else>Linux 可选择 <code>.deb</code> 或 <code>.AppImage</code>，以 Releases 页面为准。</p>
            </article>
            <article class="docs-panel">
              <div class="docs-card-head">
                <div>
                  <h3>Codex Desktop Rebuild</h3>
                  <p>便携版 Codex Desktop，主要补足插件、对话删除等体验。</p>
                </div>
                <TabButtons group="codexRebuild" :tabs="['Windows', 'macOS']" :active-tabs="activeTabs" @select="setTab" compact />
              </div>
              <div class="docs-link-grid">
                <a href="https://github.com/Haleclipse/CodexDesktop-Rebuild" target="_blank" rel="noopener noreferrer">GitHub 项目</a>
                <a href="https://github.com/Haleclipse/CodexDesktop-Rebuild/releases/tag/v26.513.31313" target="_blank" rel="noopener noreferrer">指定发布页</a>
              </div>
              <ScreenshotCard src="/docs-assets/codex-rebuild.png" alt="Codex Desktop Rebuild 截图" />
              <ol v-if="activeTabs.codexRebuild === 'Windows'">
                <li>打开发布页下载对应版本的压缩包。</li>
                <li>解压之后打开文件夹。</li>
                <li>双击 <code>Codex.exe</code> 使用。</li>
              </ol>
              <p v-else>macOS 用户优先使用官方 Codex App；如果发布页提供 macOS 包，再按发布页说明安装。</p>
            </article>
          </div>
        </section>

        <section id="faq" class="docs-section">
          <div class="docs-section-heading">
            <p class="docs-eyebrow">Troubleshooting</p>
            <h2>常见问题</h2>
            <p>延迟、用量、常见错误和反馈模板集中在这里。</p>
          </div>
          <div class="docs-two-col">
            <article class="docs-panel">
              <h3>延迟与速度</h3>
              <p>流式输出可以更快看到第一段内容，但完整生成仍需要时间。影响速度的因素包括模型大小、输入长度、输出长度、深度推理、长上下文、Agent 工具调用、服务调度和网络状态。</p>
              <ul>
                <li>简短问答通常较快。</li>
                <li>长文档总结、代码生成、Agent 连续执行会更慢。</li>
                <li>输出几千 token 时，完整耗时达到几十秒是正常现象。</li>
              </ul>
            </article>
            <article class="docs-panel">
              <h3>计费与用量</h3>
              <p>请以后台展示为准。一般会根据模型、输入 token、输出 token、缓存、图片、长上下文、特殊能力、分组倍率或套餐规则计算消耗。</p>
              <ScreenshotCard src="/docs-assets/usage-records.png" alt="用户使用记录截图" />
            </article>
          </div>
          <div class="docs-accordion">
            <details v-for="item in faqItems" :key="item.title" :open="item.open">
              <summary>{{ item.title }}</summary>
              <p>{{ item.text }}</p>
            </details>
          </div>
          <CodeBlock
            label="反馈模板"
            :code="feedbackTemplate"
            copy-id="feedback-template"
            :copied-key="copiedKey"
            @copy="copyCode"
          />
        </section>
      </main>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, reactive, ref } from 'vue'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getPublicGatewayBaseURL } from '@/api/url'

type Platform = 'Windows' | 'macOS' | 'Linux'
type ActiveTabs = Record<string, string>

const copiedKey = ref('')
const docsAssetVersion = '20260705-sanitized-4'
const apiBaseUrl = getPublicGatewayBaseURL()
const keysUrl = `${window.location.origin.replace(/\/+$/, '')}/keys`
const baseUrlAndApiKey = `Base URL: ${apiBaseUrl}\nAPI Key: sk-xxxxxx`
const activeTabs = reactive<ActiveTabs>({
  node: 'Windows',
  git: 'Windows',
  tool: 'Codex',
  codexMode: 'App',
  codexPlatform: 'Windows',
  claudeMode: 'App',
  claudePlatform: 'Windows',
  cliTool: 'Claude Code',
  cliPlatform: 'Windows',
  sessionViewer: 'Windows',
  codexRebuild: 'Windows',
})

const tocItems = [
  { id: 'overview', kicker: 'Overview', label: '总览' },
  { id: 'quick-start', kicker: 'Start', label: '快速开始' },
  { id: 'account', kicker: 'Account', label: '账号与 Key' },
  { id: 'environment', kicker: 'Env', label: '基础环境' },
  { id: 'tool-install', kicker: 'Tools', label: '工具安装' },
  { id: 'api-config', kicker: 'Config', label: 'API 配置' },
  { id: 'ccswitch', kicker: 'Switch', label: 'CCSwitch 使用' },
  { id: 'cli-config', kicker: 'CLI', label: 'CLI 配置教程' },
  { id: 'third-party', kicker: 'OpenAI', label: '第三方接入' },
  { id: 'recommendations', kicker: 'Extras', label: '工具推荐' },
  { id: 'faq', kicker: 'FAQ', label: '常见问题' },
]

const pathCards = [
  { step: '01', title: '注册并创建 Key', text: '完成账号注册，进入后台创建专属 API Key。每个软件建议单独使用一个 Key。', href: '#account', link: '查看步骤' },
  { step: '02', title: '准备基础环境', text: '安装 Node.js 与 Git，确保 Codex CLI、Claude Code CLI、ZCF、CCSwitch 等工具可运行。', href: '#environment', link: '安装环境' },
  { step: '03', title: '选择工具形态', text: '按 App、CLI、IDE 三类选择 Codex 或 Claude Code 的使用方式。', href: '#tool-install', link: '安装工具' },
  { step: '04', title: '连接 TogoAPI API', text: '使用 Base URL 和 API Key 接入，可用 CCSwitch 统一管理多个工具的供应商配置。', href: '#api-config', link: '开始配置' },
]

const quickSteps = [
  '注册 TogoAPI 账号。',
  '登录用户后台。',
  '创建专属 API Key。',
  '安装 Node.js、Git 等基础环境。',
  '使用 CCSwitch、ZCF 或其他客户端配置工具接入。',
]

const cliPaths: Record<Platform, { settings: string; config: string; auth: string }> = {
  Windows: {
    settings: '%USERPROFILE%\\.claude\\settings.json',
    config: '%USERPROFILE%\\.codex\\config.toml',
    auth: '%USERPROFILE%\\.codex\\auth.json',
  },
  macOS: {
    settings: '~/.claude/settings.json',
    config: '~/.codex/config.toml',
    auth: '~/.codex/auth.json',
  },
  Linux: {
    settings: '~/.claude/settings.json',
    config: '~/.codex/config.toml',
    auth: '~/.codex/auth.json',
  },
}

const cliPlatform = computed(() => activeTabs.cliPlatform as Platform)

const settingsJson = `{
  "env": {
    "ANTHROPIC_BASE_URL": "${apiBaseUrl}",
    "ANTHROPIC_AUTH_TOKEN": "sk-xxxxxx"
  }
}`

const configToml = `[model_providers.jqcode]
name = "TogoAPI"
base_url = "${apiBaseUrl}/v1"
wire_api = "responses"
env_key = "OPENAI_API_KEY"

model_provider = "jqcode"
model = "gpt-5.5"`

const authJson = `{
  "OPENAI_API_KEY": "sk-xxxxxx",
  "OPENAI_BASE_URL": "${apiBaseUrl}"
}`

const curlExample = `curl ${apiBaseUrl}/v1/chat/completions \\
  -H "Authorization: Bearer sk-xxxxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-5.5",
    "messages": [
      { "role": "user", "content": "你好" }
    ]
  }'`

const feedbackTemplate = `使用时间：
使用软件：
Base URL：
模型：
接口类型：/v1/chat/completions 或 /responses
错误截图：
错误文本、报错信息：
已经做过的尝试以及推断：`

const faqItems = [
  { title: '401 Invalid API key', text: '常见原因：API Key 填错、Key 前后多了空格或换行、Key 已删除或过期、Authorization 格式不正确。', open: true },
  { title: 'API key is required', text: '请求没有带 Key。检查软件里是否填写 API Key，Header 是否包含 Authorization: Bearer，以及 Key 是否填到了错误位置。' },
  { title: '403 group does not allow dispatch', text: '当前 Key 所属分组不允许使用该接口。可换用支持该接口的分组，或联系管理员确认当前分组是否开放。' },
  { title: '404 model not found', text: '常见原因：模型名称拼错、当前分组不支持该模型、模型已下线或临时维护。请复制后台显示的模型名。' },
  { title: '429 rate limit', text: '请求频率过高、服务资源繁忙或额度达到限制。降低并发、稍后重试、换模型或分组。' },
  { title: '503 no available account', text: '系统暂时没有可用的调度资源。稍后重试、换模型测试，或联系管理员确认服务配置状态。' },
  { title: 'Request timed out', text: '缩短输入、限制输出、开启流式输出，或将客户端超时时间调高到 120 秒以上。' },
  { title: '中文变成问号或乱码', text: '通常是本地命令行编码问题。建议使用支持 UTF-8 的终端，并将 JSON 文件保存为 UTF-8。' },
]

function setTab(group: string, value: string) {
  activeTabs[group] = value
}

function escapeHtml(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function detectCodeLanguage(label: string, code: string) {
  const lowerLabel = label.toLowerCase()
  const trimmed = code.trim()

  if (lowerLabel.includes('json') || trimmed.startsWith('{')) return 'json'
  if (lowerLabel.includes('toml') || trimmed.startsWith('[')) return 'toml'
  if (lowerLabel.includes('路径') || /(^|[\n\r])(?:[A-Z]:\\|~\/|\/home\/|\/Users\/)/.test(code)) return 'path'
  if (/^(?:curl|npm|npx|brew|winget|sudo|node|git|codex|claude|irm)\b/m.test(trimmed)) return 'shell'
  if (code.includes('Base URL:') || code.includes('API Key:')) return 'env'

  return 'text'
}

function highlightCode(code: string, label: string) {
  const language = detectCodeLanguage(label, code)
  const escaped = escapeHtml(code)

  if (language === 'json') {
    return escaped
      .replace(/(&quot;[^&]+?&quot;)(\s*:)/g, '<span class="token-key">$1</span>$2')
      .replace(/(:\s*)(&quot;.*?&quot;)/g, '$1<span class="token-string">$2</span>')
      .replace(/\b(true|false|null)\b/g, '<span class="token-literal">$1</span>')
  }

  if (language === 'toml') {
    return escaped
      .replace(/^(\[[^\]]+\])/gm, '<span class="token-section">$1</span>')
      .replace(/^([A-Za-z0-9_.-]+)(\s*=)/gm, '<span class="token-key">$1</span>$2')
      .replace(/(=\s*)(&quot;.*?&quot;)/g, '$1<span class="token-string">$2</span>')
  }

  if (language === 'shell') {
    return escaped
      .replace(/^([a-z][\w-]*)/gim, '<span class="token-command">$1</span>')
      .replace(/\s(--?[A-Za-z0-9][\w-]*)/g, ' <span class="token-flag">$1</span>')
      .replace(/(https?:\/\/[^\s&]+)/g, '<span class="token-url">$1</span>')
  }

  if (language === 'path') {
    return escaped.replace(/([A-Z]:\\[^\n]+|~\/[^\n]+|\/(?:home|Users)\/[^\n]+)/g, '<span class="token-path">$1</span>')
  }

  return escaped
    .replace(/(https?:\/\/[^\s]+)/g, '<span class="token-url">$1</span>')
    .replace(/\b(sk-xxxxxx)\b/g, '<span class="token-secret">$1</span>')
}

async function copyCode(code: string, key: string) {
  try {
    await navigator.clipboard.writeText(code)
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = code
    textarea.setAttribute('readonly', 'true')
    textarea.style.position = 'fixed'
    textarea.style.left = '-9999px'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
  }
  copiedKey.value = key
  window.setTimeout(() => {
    if (copiedKey.value === key) copiedKey.value = ''
  }, 1600)
}

const TabButtons = defineComponent({
  props: {
    group: { type: String, required: true },
    tabs: { type: Array<string>, required: true },
    activeTabs: { type: Object as () => ActiveTabs, required: true },
    compact: { type: Boolean, default: false },
  },
  emits: ['select'],
  setup(props, { emit }) {
    return () => h('div', { class: ['docs-tabs', props.compact ? 'compact' : ''], role: 'tablist' }, props.tabs.map((tab) =>
      h('button', {
        type: 'button',
        role: 'tab',
        'aria-selected': props.activeTabs[props.group] === tab ? 'true' : 'false',
        class: ['docs-tab-button', props.activeTabs[props.group] === tab ? 'active' : ''],
        onClick: () => emit('select', props.group, tab),
      }, tab)
    ))
  },
})

const CodeBlock = defineComponent({
  props: {
    label: { type: String, required: true },
    code: { type: String, required: true },
    copyId: { type: String, required: true },
    copiedKey: { type: String, required: true },
  },
  emits: ['copy'],
  setup(props, { emit }) {
    const languageLabel = () => detectCodeLanguage(props.label, props.code)

    return () => h('div', { class: 'docs-code-card' }, [
      h('div', { class: 'docs-code-head' }, [
        h('div', { class: 'docs-code-title' }, [
          h('span', props.label),
          h('em', { class: 'docs-code-lang' }, languageLabel()),
        ]),
        h('button', {
          type: 'button',
          'aria-label': `复制 ${props.label}`,
          onClick: () => emit('copy', props.code, props.copyId),
        }, props.copiedKey === props.copyId ? '已复制' : '复制'),
      ]),
      h('pre', [h('code', {
        class: `language-${languageLabel()}`,
        innerHTML: highlightCode(props.code, props.label),
      })]),
    ])
  },
})

const ScreenshotCard = defineComponent({
  props: {
    src: { type: String, required: true },
    alt: { type: String, required: true },
    caption: { type: String, default: '' },
    compact: { type: Boolean, default: false },
  },
  setup(props) {
    const versionedSrc = computed(() => {
      if (!props.src.startsWith('/docs-assets/')) return props.src
      const versionParam = props.src.includes('?') ? '&v=' : '?v='
      return `${props.src}${versionParam}${docsAssetVersion}`
    })

    return () => h('figure', { class: ['docs-shot', props.compact ? 'compact' : ''] }, [
      h('img', { src: versionedSrc.value, alt: props.alt, loading: 'eager' }),
      props.caption ? h('figcaption', props.caption) : null,
    ])
  },
})

const CodexInstall = defineComponent({
  props: {
    mode: { type: String, required: true },
    platform: { type: String, required: true },
    copiedKey: { type: String, required: true },
  },
  emits: ['set-platform', 'copy'],
  setup(props, { emit }) {
    const copy = (code: string, id: string) => emit('copy', code, id)
    return () => h('div', { class: 'docs-install-block' }, [
      h(TabButtons, { group: 'codexPlatform', tabs: ['Windows', 'macOS'], activeTabs, compact: true, onSelect: (_group: string, value: string) => emit('set-platform', value) }),
      props.mode === 'App'
        ? h('div', { class: 'docs-tab-panel' }, props.platform === 'Windows' ? [
          h('p', 'Codex App 适合不想长期使用终端的用户。Windows 版从 Microsoft Store 安装，安装后从开始菜单启动。'),
          h('div', { class: 'docs-link-grid' }, [
            h('a', { href: 'https://developers.openai.com/codex/app', target: '_blank', rel: 'noopener noreferrer' }, '官方文档'),
            h('a', { href: 'https://apps.microsoft.com/detail/9N6XHWQH3HGV', target: '_blank', rel: 'noopener noreferrer' }, 'Windows Microsoft Store'),
            h('a', { href: 'https://chatgpt.com/codex', target: '_blank', rel: 'noopener noreferrer' }, '网络较好时：ChatGPT Codex'),
          ]),
          h(ScreenshotCard, { src: '/docs-assets/codex-store.png', alt: 'Windows Microsoft Store 安装 Codex App 截图' }),
        ] : [
          h('p', 'macOS 先确认芯片类型，再下载对应 DMG。Apple Silicon 指 M 系列芯片；Intel 指较早期 Mac。'),
          h('div', { class: 'docs-link-grid' }, [
            h('a', { href: 'https://developers.openai.com/codex/app', target: '_blank', rel: 'noopener noreferrer' }, '官方文档'),
            h('a', { href: 'https://persistent.oaistatic.com/codex-app-prod/Codex.dmg', target: '_blank', rel: 'noopener noreferrer' }, 'macOS Apple Silicon'),
            h('a', { href: 'https://persistent.oaistatic.com/codex-app-prod/Codex-latest-x64.dmg', target: '_blank', rel: 'noopener noreferrer' }, 'macOS Intel'),
          ]),
          h(CodeBlock, { label: 'Homebrew 安装 Codex App', code: 'brew install --cask codex-app', copyId: 'codex-app-brew', copiedKey: props.copiedKey, onCopy: copy }),
        ])
        : props.mode === 'CLI'
          ? h('div', { class: 'docs-tab-panel' }, props.platform === 'Windows' ? [
            h('p', 'Windows 用户可以直接在 PowerShell 中运行 Codex。项目依赖 Linux 工具链时，再考虑 WSL2。'),
            h(CodeBlock, { label: 'npm 安装 Codex CLI', code: 'npm i -g @openai/codex', copyId: 'codex-npm-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '验证', code: 'codex --version', copyId: 'codex-version-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '启动', code: 'codex', copyId: 'codex-start-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '升级到最新版', code: 'npm i -g @openai/codex@latest', copyId: 'codex-up-win', copiedKey: props.copiedKey, onCopy: copy }),
          ] : [
            h('p', 'macOS 在“终端”中安装和使用。新手用户可继续用 npm；已经安装 Homebrew 的用户，也可以用 Homebrew 安装 Codex CLI。'),
            h(CodeBlock, { label: 'npm 安装', code: 'npm i -g @openai/codex', copyId: 'codex-npm-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'Homebrew 安装', code: 'brew install --cask codex', copyId: 'codex-brew-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '验证', code: 'codex --version', copyId: 'codex-version-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '启动', code: 'codex', copyId: 'codex-start-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'npm 更新', code: 'npm i -g @openai/codex@latest', copyId: 'codex-up-npm-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'Homebrew 更新', code: 'brew upgrade --cask codex', copyId: 'codex-up-brew-mac', copiedKey: props.copiedKey, onCopy: copy }),
          ])
          : h('div', { class: 'docs-tab-panel' }, [
            h('p', props.platform === 'Windows' ? 'Windows 常用入口是 Ctrl+Shift+X 打开扩展市场。' : 'macOS 常用 Cmd+Shift+X 打开 VS Code 扩展市场。'),
            h('div', { class: 'docs-link-grid' }, [h('a', { href: 'https://developers.openai.com/codex/ide', target: '_blank', rel: 'noopener noreferrer' }, 'Codex IDE 官方文档')]),
            h('ol', [
              h('li', '打开编辑器扩展市场。'),
              h('li', '搜索 Codex。'),
              h('li', '安装 OpenAI 官方 Codex 扩展。'),
              h('li', '安装完成后重启编辑器。'),
              h('li', '按提示登录 ChatGPT 账号或 API Key。'),
            ]),
          ]),
    ])
  },
})

const ClaudeInstall = defineComponent({
  props: {
    mode: { type: String, required: true },
    platform: { type: String, required: true },
    copiedKey: { type: String, required: true },
  },
  emits: ['set-platform', 'copy'],
  setup(props, { emit }) {
    const copy = (code: string, id: string) => emit('copy', code, id)
    return () => h('div', { class: 'docs-install-block' }, [
      h(TabButtons, { group: 'claudePlatform', tabs: ['Windows', 'macOS'], activeTabs, compact: true, onSelect: (_group: string, value: string) => emit('set-platform', value) }),
      props.mode === 'App'
        ? h('div', { class: 'docs-tab-panel' }, props.platform === 'Windows' ? [
          h('p', 'Claude 桌面应用支持 Windows 和 Windows ARM64。安装后登录 Claude 账号，在 Code 相关入口中开始使用。'),
          h('div', { class: 'docs-link-grid' }, [h('a', { href: 'https://claude.com/download', target: '_blank', rel: 'noopener noreferrer' }, 'Claude Desktop 下载页')]),
          h('ol', [h('li', '打开 Claude 下载页。'), h('li', '选择 Windows 版本；ARM64 设备选择 Windows ARM64。'), h('li', '下载并运行安装包。'), h('li', '安装完成后启动 Claude。')]),
        ] : [
          h('p', 'macOS 版可以通过 Claude 下载页安装。已经安装 Homebrew 的用户，也可以用 Homebrew 安装 Claude Desktop。'),
          h('div', { class: 'docs-link-grid' }, [h('a', { href: 'https://claude.com/download', target: '_blank', rel: 'noopener noreferrer' }, 'Claude Desktop 下载页')]),
          h(CodeBlock, { label: 'Homebrew 安装 Claude Desktop', code: 'brew install --cask claude', copyId: 'claude-app-brew', copiedKey: props.copiedKey, onCopy: copy }),
        ])
        : props.mode === 'CLI'
          ? h('div', { class: 'docs-tab-panel' }, props.platform === 'Windows' ? [
            h('p', 'Windows 原生环境推荐安装 Git for Windows。看到 PS C:\\ 说明你在 PowerShell；看到普通 C:\\ 说明你在 CMD。'),
            h(CodeBlock, { label: 'PowerShell 原生安装', code: 'irm https://claude.ai/install.ps1 | iex', copyId: 'claude-ps-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'CMD 原生安装', code: 'curl -fsSL https://claude.ai/install.cmd -o install.cmd && install.cmd && del install.cmd', copyId: 'claude-cmd-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'WinGet 安装', code: 'winget install Anthropic.ClaudeCode', copyId: 'claude-winget-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'npm 安装', code: 'npm install -g @anthropic-ai/claude-code', copyId: 'claude-npm-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '验证', code: 'claude --version', copyId: 'claude-version-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '启动', code: 'claude', copyId: 'claude-start-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '通用更新', code: 'claude update', copyId: 'claude-update-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'WinGet 更新', code: 'winget upgrade Anthropic.ClaudeCode', copyId: 'claude-winget-up-win', copiedKey: props.copiedKey, onCopy: copy }),
            h(ScreenshotCard, { src: '/docs-assets/claude-install-check.png', alt: 'Claude Code 安装成功校验截图' }),
          ] : [
            h('p', 'macOS 支持官方安装脚本、Homebrew 和 npm。官方不建议使用 sudo npm install -g 安装 Claude Code。'),
            h(CodeBlock, { label: '官方脚本安装', code: 'curl -fsSL https://claude.ai/install.sh | bash', copyId: 'claude-sh-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'Homebrew 安装', code: 'brew install --cask claude-code', copyId: 'claude-brew-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'npm 安装', code: 'npm install -g @anthropic-ai/claude-code', copyId: 'claude-npm-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '验证', code: 'claude --version', copyId: 'claude-version-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '启动', code: 'claude', copyId: 'claude-start-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: '通用更新', code: 'claude update', copyId: 'claude-update-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'Homebrew 更新', code: 'brew upgrade --cask claude-code', copyId: 'claude-brew-up-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(CodeBlock, { label: 'npm 更新', code: 'npm install -g @anthropic-ai/claude-code@latest', copyId: 'claude-npm-up-mac', copiedKey: props.copiedKey, onCopy: copy }),
            h(ScreenshotCard, { src: '/docs-assets/claude-install-check.png', alt: 'Claude Code 安装成功校验截图' }),
          ])
          : h('div', { class: 'docs-tab-panel' }, [
            h('p', props.platform === 'Windows' ? 'Windows 先装好 Claude Code CLI，再安装 IDE 扩展。VS Code 系列要求 VS Code 1.98.0 或更高版本。' : 'macOS 同样先装好 Claude Code CLI，再接入编辑器。'),
            h('div', { class: 'docs-link-grid' }, [h('a', { href: 'https://code.claude.com/docs/en/ide-integrations', target: '_blank', rel: 'noopener noreferrer' }, 'Claude Code IDE 官方文档')]),
            h('ol', [
              h('li', '打开编辑器扩展市场或 JetBrains Plugins。'),
              h('li', '搜索并安装 Claude Code 扩展。'),
              h('li', '重启编辑器或执行 Developer: Reload Window。'),
              h('li', '在侧边栏、状态栏或编辑器右上角打开 Claude Code。'),
              h('li', '外部终端会话可输入 /ide 连接。'),
            ]),
          ]),
    ])
  },
})
</script>

<style>
.docs-page {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 24px;
  align-items: start;
}

.docs-toc {
  position: sticky;
  top: 88px;
  max-height: calc(100vh - 112px);
  overflow: auto;
  border: 1px solid rgb(207 250 254);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.88);
  padding: 10px;
}

.dark .docs-toc {
  border-color: rgb(30 41 59);
  background: rgba(15, 23, 42, 0.9);
}

.docs-toc-link {
  display: block;
  border-radius: 8px;
  padding: 10px 12px;
  color: rgb(71 85 105);
  font-size: 14px;
  font-weight: 700;
}

.docs-toc-link span {
  display: block;
  color: rgb(20 184 166);
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
}

.docs-toc-link:hover {
  background: rgb(236 254 255);
  color: rgb(8 51 68);
}

.dark .docs-toc-link {
  color: rgb(203 213 225);
}

.dark .docs-toc-link:hover {
  background: rgb(22 78 99 / 0.35);
  color: white;
}

.docs-main {
  min-width: 0;
  max-width: 1180px;
}

.docs-hero,
.docs-section,
.docs-path-grid {
  border: 1px solid rgb(207 250 254);
  border-radius: 8px;
  background: white;
  box-shadow: 0 14px 40px rgba(15, 23, 42, 0.06);
}

.dark .docs-hero,
.dark .docs-section,
.dark .docs-path-grid {
  border-color: rgb(30 41 59);
  background: rgb(15 23 42);
}

.docs-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: 24px;
  padding: 32px;
  background:
    linear-gradient(135deg, rgba(240, 253, 250, 0.95), rgba(255, 255, 255, 0.98)),
    white;
}

.dark .docs-hero {
  background: linear-gradient(135deg, rgba(15, 118, 110, 0.18), rgba(15, 23, 42, 0.98));
}

.docs-eyebrow {
  margin: 0 0 8px;
  color: rgb(13 148 136);
  font-size: 12px;
  font-weight: 900;
  text-transform: uppercase;
}

.docs-hero h1 {
  margin: 0;
  color: rgb(15 23 42);
  font-size: 38px;
  font-weight: 900;
  line-height: 1.15;
}

.dark .docs-hero h1,
.dark .docs-section h2,
.dark .docs-panel h3,
.dark .docs-card h2 {
  color: white;
}

.docs-lead {
  margin: 16px 0 0;
  max-width: 760px;
  color: rgb(71 85 105);
  font-size: 16px;
  line-height: 1.8;
}

.dark .docs-lead,
.dark .docs-section-heading p,
.dark .docs-panel,
.dark .docs-card p {
  color: rgb(203 213 225);
}

.docs-actions,
.docs-link-grid,
.docs-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.docs-tabs {
  align-items: center;
}

.docs-actions {
  margin-top: 22px;
}

.docs-button,
.docs-tab-button,
.docs-code-head button,
.docs-endpoint-card button {
  border: 1px solid rgb(125 211 252);
  border-radius: 8px;
  background: white;
  color: rgb(14 116 144);
  font-weight: 800;
  transition: all 0.16s ease;
}

.docs-tab-button {
  min-width: 72px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}

.docs-tab-button:hover {
  border-color: rgb(20 184 166);
  background: rgb(240 253 250);
  color: rgb(15 118 110);
}

.docs-tab-button.active {
  box-shadow: 0 8px 18px rgba(13, 148, 136, 0.2);
}

.docs-button {
  padding: 10px 14px;
}

.docs-button.primary,
.docs-tab-button.active,
.docs-code-head button:hover,
.docs-endpoint-card button:hover {
  background: rgb(13 148 136);
  color: white;
  border-color: rgb(13 148 136);
}

.docs-endpoint-card {
  align-self: stretch;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.74);
  padding: 20px;
}

.dark .docs-endpoint-card {
  background: rgba(15, 23, 42, 0.62);
}

.docs-endpoint-card span {
  color: rgb(100 116 139);
  font-size: 13px;
  font-weight: 800;
}

.docs-endpoint-card code {
  white-space: normal;
  overflow-wrap: anywhere;
  color: rgb(15 118 110);
  font-size: 18px;
  font-weight: 900;
}

.docs-endpoint-card button,
.docs-code-head button,
.docs-tab-button {
  padding: 8px 12px;
}

.docs-path-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-top: 20px;
  padding: 18px;
}

.docs-card,
.docs-panel,
.docs-code-card {
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: rgb(248 250 252);
}

.dark .docs-card,
.dark .docs-panel,
.dark .docs-code-card {
  border-color: rgb(51 65 85);
  background: rgb(2 6 23 / 0.36);
}

.docs-card {
  padding: 18px;
}

.docs-card > span {
  display: inline-flex;
  margin-bottom: 12px;
  color: rgb(20 184 166);
  font-size: 12px;
  font-weight: 900;
}

.docs-card h2,
.docs-section h2,
.docs-panel h3 {
  margin: 0;
  color: rgb(15 23 42);
}

.docs-card p,
.docs-section-heading p,
.docs-panel p,
.docs-panel li,
.docs-accordion p {
  color: rgb(71 85 105);
  line-height: 1.75;
}

.docs-card a,
.docs-panel a,
.docs-link-grid a {
  color: rgb(8 145 178);
  font-weight: 800;
}

.docs-section {
  margin-top: 20px;
  padding: 28px;
  scroll-margin-top: 86px;
}

.docs-section-heading {
  margin-bottom: 20px;
}

.docs-section-heading h2 {
  font-size: 28px;
  font-weight: 900;
}

.docs-step-list {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}

.docs-step {
  border: 1px solid rgb(207 250 254);
  border-radius: 8px;
  background: rgb(240 253 250);
  padding: 14px;
}

.docs-step span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 999px;
  background: rgb(13 148 136);
  color: white;
  font-weight: 900;
}

.docs-step p {
  margin: 10px 0 0;
  color: rgb(15 23 42);
  font-weight: 700;
}

.docs-split,
.docs-two-col {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 18px;
  align-items: start;
}

.docs-split.reverse {
  margin-top: 18px;
}

.docs-panel {
  padding: 18px;
  min-width: 0;
}

.docs-panel h3 {
  margin-bottom: 12px;
  font-size: 20px;
  font-weight: 900;
}

.docs-panel code {
  border-radius: 6px;
  background: rgb(226 232 240);
  padding: 2px 6px;
  color: rgb(15 23 42);
  overflow-wrap: anywhere;
}

.docs-warning {
  margin-top: 14px;
  border-left: 4px solid rgb(20 184 166);
  border-radius: 8px;
  background: rgb(240 253 250);
  padding: 12px 14px;
}

.docs-shot {
  margin: 0;
  overflow: hidden;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: white;
}

.docs-shot.compact {
  max-width: 180px;
  justify-self: center;
}

.docs-shot img {
  display: block;
  width: 100%;
  height: auto;
}

.docs-shot figcaption {
  padding: 10px 12px;
  color: rgb(71 85 105);
  font-size: 13px;
  font-weight: 800;
}

.docs-image-stack,
.docs-gallery {
  display: grid;
  gap: 14px;
}

.docs-gallery {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-top: 18px;
}

.docs-table-wrap {
  margin-top: 18px;
  overflow-x: auto;
}

.docs-table-wrap table {
  width: 100%;
  min-width: 620px;
  border-collapse: collapse;
}

.docs-table-wrap th,
.docs-table-wrap td {
  border-bottom: 1px solid rgb(226 232 240);
  padding: 12px;
  text-align: left;
  color: rgb(51 65 85);
}

.docs-table-wrap th {
  background: rgb(240 253 250);
  color: rgb(15 118 110);
  font-weight: 900;
}

.docs-card-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;
  margin-bottom: 16px;
}

.docs-tabs.compact .docs-tab-button {
  padding: 6px 10px;
  font-size: 13px;
}

.docs-config-tabs {
  display: grid;
  gap: 14px;
  margin-bottom: 16px;
}

.docs-mini-label {
  margin: 0 0 8px;
  color: rgb(100 116 139);
  font-size: 12px;
  font-weight: 900;
}

.docs-config-panel {
  margin-top: 0;
}

.docs-tool-badge {
  flex: 0 0 auto;
  border: 1px solid rgb(20 184 166 / 0.32);
  border-radius: 999px;
  background: rgb(240 253 250);
  padding: 6px 10px;
  color: rgb(15 118 110);
  font-size: 12px;
  font-weight: 900;
}

.docs-tab-panel {
  display: grid;
  gap: 12px;
}

.docs-code-card {
  margin-top: 14px;
  overflow: hidden;
  border-color: rgb(30 41 59);
  background: rgb(12 18 32);
}

.docs-code-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  border-bottom: 1px solid rgb(30 41 59 / 0.78);
  background: rgb(17 24 39);
  padding: 11px 14px;
}

.docs-code-title {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.docs-code-title span {
  color: rgb(241 245 249);
  font-weight: 900;
}

.docs-code-lang {
  border: 1px solid rgb(45 212 191 / 0.35);
  border-radius: 999px;
  background: rgb(20 184 166 / 0.12);
  padding: 2px 8px;
  color: rgb(94 234 212);
  font-size: 11px;
  font-style: normal;
  font-weight: 900;
  text-transform: uppercase;
}

.docs-code-head button {
  border-color: rgb(45 212 191 / 0.45);
  background: rgb(15 23 42);
  color: rgb(153 246 228);
}

.docs-code-head button:hover {
  background: rgb(20 184 166);
  color: white;
}

.docs-code-card pre {
  margin: 0;
  overflow-x: auto;
  padding: 18px 20px;
  background:
    linear-gradient(90deg, rgb(45 212 191 / 0.06), transparent 180px),
    rgb(12 18 32);
  color: rgb(203 213 225);
}

.docs-code-card code {
  display: block;
  border-radius: 0;
  background: transparent;
  padding: 0;
  color: inherit;
  overflow-wrap: normal;
  white-space: pre;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 13.5px;
  font-weight: 500;
  line-height: 1.8;
  tab-size: 2;
}

.docs-code-card code span {
  border-radius: 0;
  background: transparent;
  padding: 0;
}

.docs-code-card ::selection {
  background: rgb(14 165 233 / 0.32);
  color: rgb(248 250 252);
}

.token-key,
.token-section {
  color: rgb(96 165 250);
}

.token-string,
.token-url,
.token-path {
  color: rgb(74 222 128);
}

.token-command {
  color: rgb(45 212 191);
  font-weight: 800;
}

.token-flag {
  color: rgb(251 191 36);
}

.token-literal,
.token-secret {
  color: rgb(196 181 253);
  font-weight: 800;
}

.docs-tool-panel {
  margin-top: 14px;
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  padding: 18px;
}

.docs-flow,
.docs-feature-grid {
  display: grid;
  gap: 12px;
}

.docs-flow {
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 18px;
}

.docs-flow div,
.docs-feature-grid span {
  border-radius: 8px;
  background: rgb(236 254 255);
  padding: 12px;
  color: rgb(8 51 68);
  font-weight: 800;
}

.docs-flow span {
  margin-right: 8px;
  color: rgb(13 148 136);
  font-weight: 900;
}

.docs-feature-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.docs-accordion {
  display: grid;
  gap: 10px;
  margin: 18px 0;
}

.docs-accordion details {
  border: 1px solid rgb(226 232 240);
  border-radius: 8px;
  background: rgb(248 250 252);
  padding: 12px 14px;
}

.docs-accordion summary {
  cursor: pointer;
  color: rgb(15 23 42);
  font-weight: 900;
}

@media (max-width: 1180px) {
  .docs-page {
    grid-template-columns: 1fr;
  }

  .docs-toc {
    position: static;
    display: flex;
    gap: 8px;
    overflow-x: auto;
    max-height: none;
  }

  .docs-toc-link {
    flex: 0 0 auto;
    min-width: 142px;
  }
}

@media (max-width: 900px) {
  .docs-hero,
  .docs-path-grid,
  .docs-step-list,
  .docs-split,
  .docs-two-col,
  .docs-flow,
  .docs-gallery {
    grid-template-columns: 1fr;
  }

  .docs-hero,
  .docs-section {
    padding: 20px;
  }

  .docs-hero h1 {
    font-size: 30px;
  }

  .docs-card-head {
    flex-direction: column;
  }
}

@media (max-width: 560px) {
  .docs-hero,
  .docs-section {
    padding: 16px;
  }

  .docs-hero h1 {
    font-size: 26px;
  }

  .docs-button,
  .docs-tab-button {
    width: 100%;
    text-align: center;
  }
}
</style>
