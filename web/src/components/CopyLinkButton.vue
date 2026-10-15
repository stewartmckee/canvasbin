<script setup>
import { ref } from 'vue'

const props = defineProps({ url: { type: String, required: true } })

const copied = ref(false)
let timer = null

async function copy() {
  try {
    await navigator.clipboard.writeText(props.url)
  } catch {
    window.prompt('Copy this link', props.url)
    return
  }
  copied.value = true
  clearTimeout(timer)
  timer = setTimeout(() => (copied.value = false), 1500)
}
</script>

<template>
  <button type="button" class="link-btn" title="Copy the read-only share link" @click="copy">
    {{ copied ? 'Copied' : 'Copy share link' }}
  </button>
</template>
