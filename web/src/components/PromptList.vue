<script setup>
import { ref } from 'vue'

defineProps({ prompts: { type: Array, required: true } })

const copied = ref('')
async function copy(p) {
  try {
    await navigator.clipboard.writeText(p)
    copied.value = p
    setTimeout(() => {
      if (copied.value === p) copied.value = ''
    }, 1500)
  } catch {
    // clipboard unavailable; the text can still be selected
  }
}
</script>

<template>
  <ul class="prompts">
    <li v-for="p in prompts" :key="p">
      <span class="prompt-text">{{ p }}</span>
      <button type="button" class="prompt-copy" :aria-label="`Copy prompt: ${p}`" @click="copy(p)">
        {{ copied === p ? 'Copied' : 'Copy' }}
      </button>
    </li>
  </ul>
</template>
