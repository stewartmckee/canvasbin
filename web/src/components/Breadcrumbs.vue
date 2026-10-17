<script setup>
import { RouterLink } from 'vue-router'

// items: [{ label, to }]; the last item is the current page and isn't linked.
defineProps({ items: { type: Array, required: true } })
</script>

<template>
  <nav class="breadcrumbs" aria-label="Breadcrumb">
    <ol>
      <li v-for="(item, i) in items" :key="item.label">
        <RouterLink v-if="i < items.length - 1" :to="item.to">{{ item.label }}</RouterLink>
        <span v-else aria-current="page">{{ item.label }}</span>
      </li>
    </ol>
  </nav>
</template>
