<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { addComment, deleteComment, getPageActivity, listComments } from '../api.js'
import { describeActivity, formatWhen } from '../activity.js'

const props = defineProps({ page: { type: Object, required: true } })

const comments = ref([])
const activity = ref([])
const body = ref('')
const busy = ref(false)
const error = ref('')

async function refreshActivity() {
  activity.value = (await getPageActivity(props.page.read_key)).activity
}

onMounted(async () => {
  try {
    comments.value = (await listComments(props.page.read_key)).comments
    await refreshActivity()
  } catch (e) {
    error.value = e.message
  }
})

async function post() {
  busy.value = true
  error.value = ''
  try {
    comments.value.push(await addComment(props.page.read_key, body.value))
    body.value = ''
    await refreshActivity()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function remove(c) {
  if (!window.confirm('Delete this comment?')) return
  error.value = ''
  try {
    await deleteComment(props.page.read_key, c.id)
    comments.value = comments.value.filter((x) => x.id !== c.id)
  } catch (e) {
    error.value = e.message
  }
}

// Refresh the timeline after the editor saves.
defineExpose({ refreshActivity })
</script>

<template>
  <div class="home-grid mt">
    <section class="panel">
      <h2>Comments</h2>
      <p class="muted small">
        Only members of <RouterLink :to="`/teams/${page.team_id}`">{{ page.team_name }}</RouterLink> see comments.
      </p>
      <p v-if="error" class="field-error">{{ error }}</p>
      <ul v-if="comments.length" class="comments">
        <li v-for="c in comments" :key="c.id">
          <strong>{{ c.author_name || c.author_email || 'Former member' }}</strong>
          <span class="muted small"> · {{ formatWhen(c.created_at) }}</span>
          <button v-if="c.can_delete" type="button" class="link-btn small" @click="remove(c)">Delete</button>
          <p class="comment-body">{{ c.body }}</p>
        </li>
      </ul>
      <p v-else class="muted">No comments yet.</p>
      <form class="comment-form" @submit.prevent="post">
        <textarea v-model="body" placeholder="Add a comment" aria-label="Comment" maxlength="4000" required />
        <button class="btn small" type="submit" :disabled="busy || !body.trim()">Comment</button>
      </form>
    </section>
    <section class="panel">
      <h2>Activity</h2>
      <p v-if="!activity.length" class="muted">No activity yet.</p>
      <ul v-else class="timeline">
        <li v-for="a in activity" :key="a.id">
          {{ describeActivity(a) }}
          <div class="muted small">{{ formatWhen(a.created_at) }}</div>
        </li>
      </ul>
    </section>
  </div>
</template>
