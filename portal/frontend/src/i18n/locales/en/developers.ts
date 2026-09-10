export default {
  developers: {
    title: 'Developer Tools',
    eyebrow: 'For developers and AI agents',
    description: 'Connect TogoAPI to terminals, applications, and automated workflows through an OpenAI-compatible API.',
    quickStart: 'Quick start',
    steps: {
      createKey: 'Create an API key',
      installSdk: 'Install the OpenAI SDK',
      makeRequest: 'Make your first request',
    },
    openKeys: 'Open API keys',
    integrations: 'Integrations',
    cards: {
      openai: {
        tag: 'SDK',
        title: 'OpenAI SDK',
        body: 'Keep using the official OpenAI Python or Node.js SDK by changing only the base URL and API key.',
      },
      curl: {
        tag: 'HTTP',
        title: 'REST API',
        body: 'Call the chat completions endpoint over standard HTTP from servers, scripts, and automation platforms.',
      },
      agents: {
        tag: 'AGENT',
        title: 'Coding Agents',
        body: 'Configure a shared endpoint and environment variables in coding agents that support OpenAI-compatible providers.',
      },
    },
    examples: 'Examples',
    exampleTitles: {
      python: 'Python',
      node: 'Node.js',
      curl: 'cURL',
      env: 'Environment variables',
    },
    copy: 'Copy code',
    copied: 'Copied',
    copyFailed: 'Copy failed',
    footer: 'OpenAI compatible · Streaming · Unified authentication · Precise billing',
  },
}
