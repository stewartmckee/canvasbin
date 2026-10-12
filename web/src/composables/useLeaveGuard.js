import { onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'

const MESSAGE = 'You have unsaved changes. Leave without saving?'

export function useLeaveGuard() {
  const dirty = ref(false)

  const onBeforeUnload = (e) => {
    if (!dirty.value) return
    e.preventDefault()
    e.returnValue = ''
  }
  onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', onBeforeUnload))
  onBeforeRouteLeave(() => !dirty.value || window.confirm(MESSAGE))

  return dirty
}
