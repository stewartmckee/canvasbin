<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { EditorView, basicSetup } from 'codemirror'
import { Compartment, EditorState } from '@codemirror/state'
import { html } from '@codemirror/lang-html'
import { markdown } from '@codemirror/lang-markdown'

const props = defineProps({
  modelValue: { type: String, default: '' },
  format: { type: String, default: 'html' },
  readonly: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

const host = ref(null)
const language = new Compartment()
const editable = new Compartment()
let view = null

const langFor = (format) => (format === 'markdown' ? markdown() : html())

onMounted(() => {
  view = new EditorView({
    parent: host.value,
    state: EditorState.create({
      doc: props.modelValue,
      extensions: [
        basicSetup,
        EditorView.lineWrapping,
        language.of(langFor(props.format)),
        editable.of(EditorState.readOnly.of(props.readonly)),
        EditorView.updateListener.of((u) => {
          if (u.docChanged) emit('update:modelValue', u.state.doc.toString())
        }),
      ],
    }),
  })
})

watch(
  () => props.modelValue,
  (value) => {
    if (view && value !== view.state.doc.toString()) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
    }
  },
)
watch(
  () => props.format,
  (f) => view?.dispatch({ effects: language.reconfigure(langFor(f)) }),
)
watch(
  () => props.readonly,
  (ro) => view?.dispatch({ effects: editable.reconfigure(EditorState.readOnly.of(ro)) }),
)

onBeforeUnmount(() => view?.destroy())
</script>

<template>
  <div ref="host" class="source" />
</template>
