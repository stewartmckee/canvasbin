<script setup>
import HomeView from './HomeView.vue'
import LandingView from './LandingView.vue'
import FoghornLandingView from './FoghornLandingView.vue'
import PagesView from './PagesView.vue'
import { accountsEnabled, auth, currentUser } from '../auth.js'
</script>

<template>
  <HomeView v-if="!accountsEnabled" />
  <PagesView v-else-if="currentUser" />
  <FoghornLandingView v-else-if="auth.signalProduct" />
  <LandingView v-else />
</template>
