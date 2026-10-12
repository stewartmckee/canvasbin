<script setup>
import { onBeforeUnmount, ref, watch } from 'vue'
import { renderPreview } from '../api.js'

const props = defineProps({
  title: { type: String, default: '' },
  format: { type: String, default: 'html' },
  content: { type: String, default: '' },
})

const doc = ref('')
const error = ref('')
let timer = null
let seq = 0

async function refresh() {
  const mine = ++seq
  try {
    const res = await renderPreview({ title: props.title, format: props.format, content: props.content })
    if (mine === seq) {
      doc.value = res.html
      error.value = ''
    }
  } catch (e) {
    if (mine === seq) error.value = e.message
  }
}

watch(
  () => [props.title, props.format, props.content],
  () => {
    clearTimeout(timer)
    timer = setTimeout(refresh, 250)
  },
  { immediate: true },
)

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div class="preview">
    <p v-if="error" class="notice error">{{ error }}</p>
    <iframe
      title="Page preview"
      :srcdoc="doc"
      sandbox="allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-modals"
      referrerpolicy="no-referrer"
    />
  </div>
</template>
