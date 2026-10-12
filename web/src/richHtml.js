const RICH_HTML =
  /<(script|style|div|section|svg|canvas|iframe|form|input|button|select|textarea|header|footer|nav|main|article|aside|video|audio|details|figure)\b|\sstyle\s*=|\sclass\s*=/i

// needsSource reports HTML the visual editor would simplify: full documents and
// markup with layout, styling or scripts.
export function needsSource(content, format) {
  if (format !== 'html' || !content) return false
  if (/^\s*(<!--[\s\S]*?-->\s*)*(<!doctype\s|<html[\s>])/i.test(content)) return true
  return RICH_HTML.test(content)
}
