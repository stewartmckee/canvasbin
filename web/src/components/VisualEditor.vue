<script setup>
import { onBeforeUnmount, watch } from 'vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import { extensions, contentOptions, serialize } from '../tiptap.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  format: { type: String, default: 'html' },
  readonly: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

let lastEmitted = props.modelValue

const editor = useEditor({
  extensions: extensions(),
  content: props.modelValue,
  ...contentOptions(props.format),
  editable: !props.readonly,
  onUpdate: ({ editor }) => {
    lastEmitted = serialize(editor, props.format)
    emit('update:modelValue', lastEmitted)
  },
})

watch(
  () => [props.modelValue, props.format],
  ([value, format], [, oldFormat]) => {
    if (!editor.value) return
    if (value === lastEmitted && format === oldFormat) return
    lastEmitted = value
    editor.value.commands.setContent(value, { emitUpdate: false, ...contentOptions(format) })
  },
)

watch(
  () => props.readonly,
  (ro) => editor.value?.setEditable(!ro),
)

onBeforeUnmount(() => editor.value?.destroy())

function run(fn) {
  if (editor.value) fn(editor.value.chain().focus()).run()
}

function isActive(name, attrs) {
  return editor.value?.isActive(name, attrs) ?? false
}

function setLink() {
  const prev = editor.value?.getAttributes('link').href || ''
  const url = window.prompt('Link URL (leave empty to remove)', prev)
  if (url === null) return
  if (url.trim() === '') run((c) => c.extendMarkRange('link').unsetLink())
  else run((c) => c.extendMarkRange('link').setLink({ href: url.trim() }))
}

function addImage() {
  const src = window.prompt('Image URL')
  if (src?.trim()) run((c) => c.setImage({ src: src.trim() }))
}

const buttons = [
  { label: 'B', title: 'Bold', cls: 'b', active: () => isActive('bold'), act: (c) => c.toggleBold() },
  { label: 'I', title: 'Italic', cls: 'i', active: () => isActive('italic'), act: (c) => c.toggleItalic() },
  { label: 'S', title: 'Strikethrough', cls: 's', active: () => isActive('strike'), act: (c) => c.toggleStrike() },
  { label: '</>', title: 'Inline code', active: () => isActive('code'), act: (c) => c.toggleCode() },
  'sep',
  { label: 'H1', title: 'Heading 1', active: () => isActive('heading', { level: 1 }), act: (c) => c.toggleHeading({ level: 1 }) },
  { label: 'H2', title: 'Heading 2', active: () => isActive('heading', { level: 2 }), act: (c) => c.toggleHeading({ level: 2 }) },
  { label: 'H3', title: 'Heading 3', active: () => isActive('heading', { level: 3 }), act: (c) => c.toggleHeading({ level: 3 }) },
  'sep',
  { label: '• List', title: 'Bullet list', active: () => isActive('bulletList'), act: (c) => c.toggleBulletList() },
  { label: '1. List', title: 'Numbered list', active: () => isActive('orderedList'), act: (c) => c.toggleOrderedList() },
  { label: '❝', title: 'Quote', active: () => isActive('blockquote'), act: (c) => c.toggleBlockquote() },
  { label: '{ }', title: 'Code block', active: () => isActive('codeBlock'), act: (c) => c.toggleCodeBlock() },
  'sep',
  { label: 'Link', title: 'Link', active: () => isActive('link'), click: setLink },
  { label: 'Image', title: 'Insert image from URL', click: addImage },
  { label: 'Table', title: 'Insert table', act: (c) => c.insertTable({ rows: 3, cols: 3, withHeaderRow: true }) },
  { label: '―', title: 'Horizontal rule', act: (c) => c.setHorizontalRule() },
  'sep',
  { label: '↶', title: 'Undo', act: (c) => c.undo() },
  { label: '↷', title: 'Redo', act: (c) => c.redo() },
]
</script>

<template>
  <div class="visual">
    <div v-if="!readonly && editor" class="toolbar" role="toolbar" aria-label="Formatting">
      <template v-for="(b, i) in buttons" :key="i">
        <span v-if="b === 'sep'" class="sep" />
        <button
          v-else
          type="button"
          :class="[b.cls, { active: b.active?.() }]"
          :title="b.title"
          :aria-label="b.title"
          @mousedown.prevent
          @click="b.click ? b.click() : run(b.act)"
        >
          {{ b.label }}
        </button>
      </template>
      <template v-if="isActive('table')">
        <span class="sep" />
        <button type="button" title="Add row" @mousedown.prevent @click="run((c) => c.addRowAfter())">+Row</button>
        <button type="button" title="Add column" @mousedown.prevent @click="run((c) => c.addColumnAfter())">+Col</button>
        <button type="button" title="Delete row" @mousedown.prevent @click="run((c) => c.deleteRow())">−Row</button>
        <button type="button" title="Delete column" @mousedown.prevent @click="run((c) => c.deleteColumn())">−Col</button>
        <button type="button" title="Delete table" @mousedown.prevent @click="run((c) => c.deleteTable())">✕Table</button>
      </template>
    </div>
    <EditorContent :editor="editor" class="prose" />
  </div>
</template>
