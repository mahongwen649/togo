import DOMPurify from 'dompurify'
import katex from 'katex'
import { marked } from 'marked'

const mathPattern = /\\\[([\s\S]*?)\\\]|\$\$([\s\S]*?)\$\$|\\\(([\s\S]*?)\\\)/g

function escapeHTML(value: string): string {
  return value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

export function renderAccountCheckMarkdown(source: string): string {
  const formulas: string[] = []
  const protectedSource = source.replace(mathPattern, (_match, bracketBlock: string | undefined, dollarBlock: string | undefined, inline: string | undefined) => {
    const expression = bracketBlock ?? dollarBlock ?? inline ?? ''
    const displayMode = inline === undefined
    const index = formulas.push(katex.renderToString(expression.trim(), {
      displayMode,
      throwOnError: false,
      strict: 'ignore',
      trust: false,
      output: 'htmlAndMathml'
    })) - 1
    return `ACCOUNT_CHECK_MATH_${index}_TOKEN`
  })
  const markdown = marked.parse(escapeHTML(protectedSource), { breaks: true, gfm: true }) as string
  const withMath = markdown.replace(/ACCOUNT_CHECK_MATH_(\d+)_TOKEN/g, (_match, index: string) => formulas[Number(index)] || '')
  return DOMPurify.sanitize(withMath)
}
