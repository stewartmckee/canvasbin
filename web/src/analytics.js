// Umami tracking. The script loads deferred with auto-track off (see
// index.html), so calls made before it arrives are queued.
const queue = []
let waiting = false

function flush() {
  if (window.umami) {
    while (queue.length) queue.shift()()
    return
  }
  if (!waiting) {
    waiting = true
    let tries = 0
    const timer = setInterval(() => {
      if (window.umami || ++tries > 50) {
        clearInterval(timer)
        waiting = false
        if (window.umami) flush()
        else queue.length = 0
      }
    }, 200)
  }
}

function enqueue(fn) {
  if (import.meta.env.SSR) return
  queue.push(fn)
  flush()
}

// Captures the path and title now, so views queued before the script loads
// are still attributed to the right page.
export function pageview() {
  const url = location.pathname + location.search
  const title = document.title
  enqueue(() => window.umami.track((props) => ({ ...props, url, title })))
}

export function track(event, data) {
  enqueue(() => window.umami.track(event, data))
}
