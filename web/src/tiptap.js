import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { TableKit } from '@tiptap/extension-table'
import Image from '@tiptap/extension-image'
import { Markdown } from '@tiptap/markdown'

export function extensions() {
  return [
    StarterKit.configure({
      link: { openOnClick: false, autolink: true, defaultProtocol: 'https' },
    }),
    TableKit.configure({ table: { resizable: false } }),
    Image,
    Markdown,
  ]
}

export function contentOptions(format) {
  return format === 'markdown' ? { contentType: 'markdown' } : { contentType: 'html' }
}

export function serialize(editor, format) {
  return format === 'markdown' ? editor.getMarkdown() : editor.getHTML()
}

// convert re-expresses content in the other format via a headless editor.
export function convert(content, from, to) {
  if (from === to || !content.trim()) return content
  const editor = new Editor({ extensions: extensions(), content, ...contentOptions(from) })
  try {
    return serialize(editor, to)
  } finally {
    editor.destroy()
  }
}