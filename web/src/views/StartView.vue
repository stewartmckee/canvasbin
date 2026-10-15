<script setup>
import HomeView from './HomeView.vue'
import LandingView from './LandingView.vue'
import PagesView from './PagesView.vue'
import { accountsEnabled, currentUser } from '../auth.js'
</script>

<template>
  <HomeView v-if="!accountsEnabled" />
  <PagesView v-else-if="currentUser" />
  <LandingView v-else />
</template>
