export default {
  developers: {
    title: '开发者工具',
    eyebrow: '面向开发者与 AI Agent',
    description: '通过 OpenAI 兼容接口，将 TogoAPI 接入终端、应用或自动化工作流。',
    quickStart: '快速开始',
    steps: {
      createKey: '创建 API 密钥',
      installSdk: '安装 OpenAI SDK',
      makeRequest: '发起首个请求',
    },
    openKeys: '前往密钥管理',
    integrations: '接入方式',
    cards: {
      openai: {
        tag: 'SDK',
        title: 'OpenAI SDK',
        body: '只需替换 Base URL 和 API Key，即可继续使用 OpenAI 官方 Python 或 Node.js SDK。',
      },
      curl: {
        tag: 'HTTP',
        title: 'REST API',
        body: '使用标准 HTTP 请求调用聊天补全接口，适合服务端、脚本和自动化平台。',
      },
      agents: {
        tag: 'AGENT',
        title: 'Coding Agents',
        body: '为支持 OpenAI 兼容供应商的 Coding Agent 配置统一端点和环境变量。',
      },
    },
    examples: '示例',
    exampleTitles: {
      python: 'Python',
      node: 'Node.js',
      curl: 'cURL',
      env: '环境变量',
    },
    copy: '复制代码',
    copied: '已复制',
    copyFailed: '复制失败',
    footer: 'OpenAI 兼容 · 流式响应 · 统一鉴权 · 精确计费',
  },
}
