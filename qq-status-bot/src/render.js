import sharp from 'sharp'

const escapeXml = value => String(value ?? '')
  .replaceAll('&', '&amp;')
  .replaceAll('<', '&lt;')
  .replaceAll('>', '&gt;')
  .replaceAll('"', '&quot;')

function statusColor(status) {
  return status === 'operational' ? '#18b66a' : status === 'degraded' ? '#e9a11b' : '#e34b4b'
}

export async function renderStatus(items, now = new Date()) {
  const width = 960
  const columns = 2
  const cardWidth = 436
  const cardHeight = 148
  const rows = Math.max(1, Math.ceil(items.length / columns))
  const height = 150 + rows * (cardHeight + 20) + 34
  const cards = items.map((item, index) => {
    const column = index % columns
    const row = Math.floor(index / columns)
    const x = 32 + column * (cardWidth + 24)
    const y = 116 + row * (cardHeight + 20)
    const availability = Number(item.availability_7d || 0).toFixed(1)
    const latency = item.primary_latency_ms == null ? '--' : `${Math.round(item.primary_latency_ms)} ms`
    const color = statusColor(item.primary_status)
    return `<g transform="translate(${x} ${y})">
      <rect width="${cardWidth}" height="${cardHeight}" rx="8" fill="#ffffff" stroke="#dce4e8"/>
      <circle cx="24" cy="28" r="6" fill="${color}"/>
      <text x="40" y="35" class="name">${escapeXml(item.name)}</text>
      <text x="24" y="68" class="meta">${escapeXml(item.primary_model || item.group_name || '')}</text>
      <text x="24" y="112" class="big" fill="${color}">${availability}%</text>
      <text x="24" y="134" class="label">7 天可用率</text>
      <text x="${cardWidth - 24}" y="112" text-anchor="end" class="latency">${latency}</text>
      <text x="${cardWidth - 24}" y="134" text-anchor="end" class="label">最近延迟</text>
    </g>`
  }).join('')

  const healthy = items.filter(item => item.primary_status === 'operational').length
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}">
    <rect width="100%" height="100%" fill="#f4f8f9"/>
    <style>
      text { font-family: "Microsoft YaHei", "Noto Sans CJK SC", sans-serif; }
      .title { font-size: 30px; font-weight: 700; fill: #15282d; }
      .summary { font-size: 16px; fill: #587078; }
      .name { font-size: 19px; font-weight: 700; fill: #193239; }
      .meta, .label { font-size: 13px; fill: #71858b; }
      .big { font-size: 27px; font-weight: 700; }
      .latency { font-size: 20px; font-weight: 600; fill: #314a51; }
    </style>
    <text x="32" y="50" class="title">站点状态检查</text>
    <text x="32" y="82" class="summary">${healthy}/${items.length} 个监控正常 · ${escapeXml(now.toLocaleString('zh-CN', { hour12: false }))}</text>
    ${cards || '<text x="32" y="150" class="summary">暂无监控数据</text>'}
  </svg>`
  return sharp(Buffer.from(svg)).png().toBuffer()
}
