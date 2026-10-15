<script setup>
import { ref } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  value: { type: String, required: true },
  hint: { type: String, default: '' },
  tone: { type: String, default: 'neutral' },
  openable: { type: Boolean, default: true },
})

const copied = ref(false)
let timer = null

async function copy() {
  try {
    await navigator.clipboard.writeText(props.value)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = props.value
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    ta.remove()
  }
  copied.value = true
  clearTimeout(timer)
  timer = setTimeout(() => (copied.value = false), 1500)
}
</script>

<template>
  <div class="copy-field" :class="tone">
    <div class="copy-label">
      <strong>{{ label }}</strong>
      <span v-if="hint">{{ hint }}</span>
    </div>
    <div class="copy-row">
      <input :value="value" readonly :aria-label="label" @focus="$event.target.select()" />
      <button type="button" class="btn" @click="copy">{{ copied ? 'Copied' : 'Copy' }}</button>
      <a v-if="openable" class="btn ghost" :href="value" target="_blank" rel="noopener noreferrer">Open</a>
    </div>
  </div>
</template>
