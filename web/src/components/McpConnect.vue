<script setup>
import { computed, ref } from 'vue'
import CopyField from './CopyField.vue'

const props = defineProps({
  // With accounts on the connector signs in with OAuth; otherwise it is open.
  oauth: { type: Boolean, default: true },
})

const name = 'canvasbin'
const url = `${window.location.origin}/mcp`
const json = (v) => JSON.stringify(v, null, 2)

const cursorLink = `cursor://anysphere.cursor-deeplink/mcp/install?name=${name}&config=${btoa(JSON.stringify({ url }))}`
const vscodeConfig = encodeURIComponent(JSON.stringify({ name, type: 'http', url }))
const vscodeLink = `vscode:mcp/install?${vscodeConfig}`
const vscodeInsidersLink = `vscode-insiders:mcp/install?${vscodeConfig}`

const clients = [
  { id: 'cursor', label: 'Cursor' },
  { id: 'vscode', label: 'VS Code' },
  { id: 'claude-code', label: 'Claude Code' },
  { id: 'claude', label: 'Claude' },
  { id: 'chatgpt', label: 'ChatGPT' },
  { id: 'codex', label: 'Codex' },
  { id: 'gemini', label: 'Gemini CLI' },
  { id: 'windsurf', label: 'Windsurf' },
  { id: 'other', label: 'Other' },
]
const tab = ref('cursor')

const signInNote = computed(() =>
  props.oauth ? 'The first time it publishes a page, your assistant asks you to sign in to Canvasbin.' : '',
)
</script>

<template>
  <div class="mcp-connect">
    <nav class="tabs" role="tablist">
      <button
        v-for="c in clients"
        :key="c.id"
        type="button"
        role="tab"
        :aria-selected="tab === c.id"
        :class="{ on: tab === c.id }"
        @click="tab = c.id"
      >
        {{ c.label }}
      </button>
    </nav>

    <div class="mcp-body" role="tabpanel">
      <template v-if="tab === 'cursor'">
        <p>Install Canvasbin in Cursor with one click.</p>
        <a class="btn primary" :href="cursorLink">Add to Cursor</a>
        <p class="muted small">Or add it to <code>~/.cursor/mcp.json</code>:</p>
        <pre class="snippet">{{ json({ mcpServers: { [name]: { url } } }) }}</pre>
      </template>

      <template v-else-if="tab === 'vscode'">
        <p>Install Canvasbin in VS Code (GitHub Copilot agent mode) with one click.</p>
        <div class="notice-actions">
          <a class="btn primary" :href="vscodeLink">Add to VS Code</a>
          <a class="btn" :href="vscodeInsidersLink">Add to VS Code Insiders</a>
        </div>
        <p class="muted small">Or add it to <code>.vscode/mcp.json</code>:</p>
        <pre class="snippet">{{ json({ servers: { [name]: { type: 'http', url } } }) }}</pre>
      </template>

      <template v-else-if="tab === 'claude-code'">
        <p>Run this in your terminal:</p>
        <CopyField label="Command" :value="`claude mcp add --transport http ${name} ${url}`" :openable="false" />
        <p v-if="oauth" class="muted small">Then run <code>/mcp</code> in Claude Code to sign in.</p>
      </template>

      <template v-else-if="tab === 'claude'">
        <p>In Claude (web or desktop), add Canvasbin as a custom connector:</p>
        <ol class="steps">
          <li>Open <strong>Settings → Connectors</strong> and choose <strong>Add custom connector</strong>.</li>
          <li>Name it <strong>Canvasbin</strong> and paste the URL below.</li>
          <li>Turn the connector on from the tools menu in a chat.</li>
        </ol>
        <CopyField label="Connector URL" :value="url" :openable="false" />
      </template>

      <template v-else-if="tab === 'chatgpt'">
        <p>In ChatGPT, add Canvasbin as a connector:</p>
        <ol class="steps">
          <li>Open <strong>Settings → Apps &amp; Connectors → Advanced settings</strong> and turn on <strong>Developer mode</strong>.</li>
          <li>Choose <strong>Create</strong>, name it <strong>Canvasbin</strong> and paste the URL below<span v-if="oauth">, with OAuth authentication</span>.</li>
          <li>Pick Canvasbin from the tools menu in a chat.</li>
        </ol>
        <CopyField label="Connector URL" :value="url" :openable="false" />
      </template>

      <template v-else-if="tab === 'codex'">
        <p>Run this in your terminal:</p>
        <CopyField label="Command" :value="`codex mcp add ${name} --url ${url}`" :openable="false" />
        <template v-if="oauth">
          <p class="muted small">Then sign in:</p>
          <CopyField label="Command" :value="`codex mcp login ${name}`" :openable="false" />
        </template>
      </template>

      <template v-else-if="tab === 'gemini'">
        <p>Run this in your terminal:</p>
        <CopyField label="Command" :value="`gemini mcp add --transport http ${name} ${url}`" :openable="false" />
        <p v-if="oauth" class="muted small">Then run <code>/mcp auth {{ name }}</code> in Gemini CLI to sign in.</p>
      </template>

      <template v-else-if="tab === 'windsurf'">
        <p>Add this to <code>~/.codeium/windsurf/mcp_config.json</code>, then refresh MCP servers in Cascade:</p>
        <pre class="snippet">{{ json({ mcpServers: { [name]: { serverUrl: url } } }) }}</pre>
      </template>

      <template v-else>
        <p>Canvasbin works with any client that supports remote MCP servers over Streamable HTTP.</p>
        <CopyField
          label="Server URL"
          :value="url"
          :hint="oauth ? 'Streamable HTTP with OAuth sign-in.' : 'Streamable HTTP, no auth.'"
          :openable="false"
        />
        <p class="muted small">Most clients take JSON like this:</p>
        <pre class="snippet">{{ json({ mcpServers: { [name]: { url } } }) }}</pre>
        <p class="muted small">For clients that only run local (stdio) servers, bridge with <code>mcp-remote</code>:</p>
        <pre class="snippet">{{ json({ mcpServers: { [name]: { command: 'npx', args: ['-y', 'mcp-remote', url] } } }) }}</pre>
      </template>

      <p v-if="signInNote" class="muted small">{{ signInNote }}</p>
    </div>
  </div>
</template>
