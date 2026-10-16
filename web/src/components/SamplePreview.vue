<script setup>
import { sampleUrl } from '../usecases.js'

const props = defineProps({
  sample: { type: Object, required: true },
  tall: { type: Boolean, default: false },
})
const url = sampleUrl(props.sample.key)
</script>

<template>
  <article class="sample" :class="{ tall }">
    <div class="sample-frame">
      <div class="mock-bar"><span /><span /><span /><em class="sample-url">canvasbin.dev/{{ sample.key.slice(0, 6) }}…html</em></div>
      <iframe :src="url" :title="sample.title" loading="lazy" sandbox="allow-scripts" referrerpolicy="no-referrer" />
    </div>
    <div class="sample-body">
      <h3>{{ sample.title }}</h3>
      <p class="muted">{{ sample.body }}</p>
      <p class="sample-schedule">↻ {{ sample.schedule }}</p>
      <a :href="url" target="_blank" rel="noopener">Open the live page →</a>
    </div>
  </article>
</template>
