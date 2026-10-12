<script setup>
import CopyField from './CopyField.vue'

defineProps({
  page: { type: Object, required: true },
  fresh: { type: Boolean, default: false },
})
</script>

<template>
  <section class="share" :class="{ fresh }">
    <div class="share-head">
      <h2>{{ fresh ? 'Page created — pick a link to share' : page.can_write ? 'Share' : 'Read-only page' }}</h2>
      <div class="share-actions">
        <a class="btn ghost small" :href="page.html_url" target="_blank" rel="noopener noreferrer">View page</a>
        <a class="btn ghost small" :href="page.pdf_url" download>Download PDF</a>
      </div>
    </div>
    <CopyField
      label="Read-only link"
      hint="Shows the rendered page. Swap .html for .pdf to get a PDF."
      :value="page.html_url"
      tone="read"
    />
    <CopyField
      v-if="page.write_url"
      label="Edit link"
      hint="Opens this editor. Anyone with it can change or delete the page — keep it private."
      :value="page.write_url"
      tone="write"
    />
  </section>
</template>
