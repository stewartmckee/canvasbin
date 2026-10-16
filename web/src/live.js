// Subscribes to change events for a page. onEvent receives {type, updated_at}
// for "hello" (on every connect), "updated" and "deleted". Returns a stop function.
export function watchPage(key, onEvent) {
  let ws = null
  let stopped = false
  let delay = 1000
  let timer = null

  function connect() {
    if (stopped) return
    const proto = location.protocol === 'https:' ? 'wss://' : 'ws://'
    try {
      ws = new WebSocket(`${proto}${location.host}/api/pages/${encodeURIComponent(key)}/live`)
    } catch {
      return
    }
    ws.onopen = () => {
      delay = 1000
    }
    ws.onmessage = (e) => {
      let msg
      try {
        msg = JSON.parse(e.data)
      } catch {
        return
      }
      onEvent(msg)
      if (msg.type === 'deleted') stop()
    }
    ws.onclose = () => {
      if (stopped) return
      timer = setTimeout(connect, delay)
      delay = Math.min(delay * 2, 30000)
    }
  }

  function stop() {
    stopped = true
    clearTimeout(timer)
    if (ws) {
      ws.onclose = null
      ws.close()
    }
  }

  connect()
  return stop
}
