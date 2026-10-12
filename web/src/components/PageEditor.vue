<script setup>
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import PreviewPane from './PreviewPane.vue'
import { needsSource } from '../richHtml.js'

const VisualEditor = defineAsyncComponent(() => import('./VisualEditor.vue'))
const SourceEditor = defineAsyncComponent(() => import('./SourceEditor.vue'))

const props = defineProps({
  page: { type: Object, required: true },
  mode: { type: String, default: 'edit' },
  saving: { type: Boolean, default: false },
  status: { type: String, default: '' },
})
const emit = defineEmits(['save', 'delete', 'dirty'])

const readonly = computed(() => props.mode === 'view')
const title = ref(props.page.title || '')
const format = ref(props.page.format || 'html')
const content = ref(props.page.content || '')
const baseline = ref(snapshot())
const tab = ref(readonly.value ? 'preview' : needsSource(content.value, format.value) ? 'source' : 'visual')
const forceVisual = ref(false)

function snapshot() {
  return JSON.stringify([title.value, format.value, content.value])
}

const dirty = computed(() => snapshot() !== baseline.value)
watch(dirty, (d) => emit('dirty', d), { immediate: true })

const richHtml = computed(() => needsSource(content.value, format.value))
const showVisual = computed(() => !richHtml.value || forceVisual.value)

async function changeFormat(next) {
  const prev = format.value
  if (next === prev) return
  if (content.value.trim()) {
    if (prev === 'html' && richHtml.value) {
      const ok = window.confirm(
        'Converting to Markdown keeps text, headings, lists, links, images and tables, but drops layout, styles and scripts. Continue?',
      )
      if (!ok) return
    }
    try {
      const { convert } = await import('../tiptap.js')
      content.value = convert(content.value, prev, next)
    } catch (e) {
      console.error(e)
    }
  }
  format.value = next
  forceVisual.value = false
}

function save() {
  if (props.saving || readonly.value) return
  emit('save', { title: title.value, format: format.value, content: content.value })
}

function markSaved(saved) {
  baseline.value = saved ? JSON.stringify([saved.title, saved.format, saved.content]) : snapshot()
}
defineExpose({ markSaved })

function onKey(e) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
    e.preventDefault()
    save()
  }
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

const tabs = computed(() =>
  readonly.value
    ? [
        { id: 'preview', label: 'Preview' },
        { id: 'source', label: 'Source' },
      ]
    : [
        { id: 'visual', label: 'Visual' },
        { id: 'source', label: 'Source' },
        { id: 'preview', label: 'Preview' },
      ],
)
</script>

<template>
  <section class="editor-card">
    <header class="editor-head">
      <input
        v-model="title"
        class="title-input"
        placeholder="Untitled page"
        maxlength="200"
        aria-label="Title"
        :readonly="readonly"
      />
      <div v-if="readonly" class="editor-actions">
        <span class="status">{{ format === 'html' ? 'HTML' : 'Markdown' }} · read-only</span>
      </div>
      <div v-else class="editor-actions">
        <div class="segmented" role="radiogroup" aria-label="Format">
          <button
            v-for="f in ['html', 'markdown']"
            :key="f"
            type="button"
            role="radio"
            :aria-checked="format === f"
            :class="{ on: format === f }"
            @click="changeFormat(f)"
          >
            {{ f === 'html' ? 'HTML' : 'Markdown' }}
          </button>
        </div>
        <span class="status" :class="{ warn: dirty }">{{ dirty ? 'Unsaved changes' : status }}</span>
        <button v-if="mode === 'edit'" type="button" class="btn danger ghost" @click="emit('delete')">Delete</button>
        <button type="button" class="btn primary" :disabled="saving || (mode === 'edit' && !dirty)" @click="save">
          {{ saving ? 'Saving…' : mode === 'create' ? 'Create page' : 'Save' }}
        </button>
      </div>
    </header>

    <nav class="tabs" role="tablist">
      <button
        v-for="t in tabs"
        :key="t.id"
        type="button"
        role="tab"
        :aria-selected="tab === t.id"
        :class="{ on: tab === t.id }"
        @click="tab = t.id"
      >
        {{ t.label }}
      </button>
      <span v-if="!readonly" class="tabs-hint">{{ format === 'html' ? 'HTML' : 'Markdown' }} · ⌘/Ctrl+S to save</span>
    </nav>

    <div class="editor-body">
      <template v-if="tab === 'visual'">
        <div v-if="!showVisual" class="notice">
          <p>
            This HTML has layout, styling or scripts that the visual editor would simplify. Edit it in
            <strong>Source</strong> to keep it exactly as written.
          </p>
          <div class="notice-actions">
            <button type="button" class="btn primary" @click="tab = 'source'">Open Source</button>
            <button type="button" class="btn ghost" @click="forceVisual = true">Edit visually anyway</button>
          </div>
        </div>
        <VisualEditor v-else v-model="content" :format="format" />
      </template>
      <SourceEditor v-else-if="tab === 'source'" v-model="content" :format="format" :readonly="readonly" />
      <PreviewPane v-else :title="title" :format="format" :content="content" />
    </div>
  </section>
</template>
