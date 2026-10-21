// Content for the public "Connect your AI" pages: /connect and one page per
// assistant. `tabs` picks the setup instructions shown from McpConnect.

export const connectHub = {
  seoTitle: 'Canvasbin MCP server: publish pages from Claude, ChatGPT and Cursor',
  seoDescription:
    'Connect the Canvasbin remote MCP server to Claude, ChatGPT, Cursor, VS Code or Codex, and let your AI assistant publish and update HTML and Markdown pages.',
  eyebrow: 'Remote MCP server',
  title: 'Connect your AI assistant to Canvasbin',
  lead: 'Canvasbin is a remote MCP server. Add it to Claude, ChatGPT, Cursor or any MCP client, sign in once, and your assistant can publish HTML and Markdown pages at a shareable link and keep them up to date.',
  faqs: [
    [
      'What is an MCP server?',
      'MCP (Model Context Protocol) is the open standard AI assistants use to call tools. Canvasbin runs a remote MCP server at canvasbin.dev/mcp, so any assistant that supports MCP can publish pages to your account.',
    ],
    [
      'Which assistants work with Canvasbin?',
      'Claude, Claude Code, ChatGPT, Cursor, VS Code, Codex, Gemini CLI, Windsurf and any other client that supports remote MCP servers over Streamable HTTP.',
    ],
    [
      'How does my assistant sign in?',
      'With OAuth. The first time it publishes, your assistant opens a Canvasbin sign-in page. It never sees your password, and you can disconnect it from your account page at any time.',
    ],
    [
      'What can my assistant see?',
      'Only your Canvasbin pages, the pages shared with you, your team pages, and a team’s style guide when the Team plan is active. It can’t reach anything else in your account, such as billing.',
    ],
    [
      'Does it cost anything?',
      'The MCP server is included on every plan, including Free. A team style guide, which assistants follow when they write team pages, is part of the Team plan.',
    ],
  ],
}

// Starter prompts shown on every connect page, once the assistant is connected.
export const starterPrompts = [
  'Create a new canvasbin and put a game of tic tac toe on it.',
  'Create a canvasbin with the latest agentic news, create it in a newsroom style and make it a single page application with navigation and in depth articles.',
  'Make a newspaper-style front page of today’s top stories and publish it to Canvasbin.',
  'Turn this spreadsheet into a dashboard with charts and give me a Canvasbin link and a PDF.',
  'Draw our system architecture as diagrams and publish it as a Canvasbin page.',
]

export const assistants = [
  {
    slug: 'claude',
    label: 'Claude',
    tabs: ['claude', 'claude-code'],
    seoTitle: 'Publish HTML pages from Claude with the Canvasbin MCP connector',
    seoDescription:
      'Add Canvasbin to Claude as a connector and turn Claude’s HTML, reports and artifacts into shareable web pages and PDFs that Claude can keep up to date.',
    title: 'Publish pages from Claude',
    lead: 'Add Canvasbin as a connector in Claude or Claude Code. Claude can then turn its HTML, reports and artifacts into pages with a shareable link and a PDF, and update them whenever you ask.',
    prompts: [
      'Turn this analysis into a Canvasbin page and give me the link.',
      'Publish that artifact on Canvasbin so I can send it to the team.',
      'Update my quarterly report page with this week’s numbers.',
      'List my Canvasbin pages and tidy up their titles.',
    ],
    faqs: [
      [
        'How is a Canvasbin page different from a Claude artifact?',
        'A Canvasbin page lives in your own library at a link you control. Claude can keep updating it, people viewing it see changes live, and every page has a PDF download and a private edit link.',
      ],
      [
        'Can Claude update a page after publishing it?',
        'Yes. Claude calls update_page with the page’s write key. The link stays the same and anyone viewing the page sees the new version straight away.',
      ],
      [
        'Does it work in Claude Code?',
        'Yes. Add the server with one command, run /mcp to sign in, and Claude Code can publish release notes, diagrams and reports from your repo.',
      ],
    ],
  },
  {
    slug: 'chatgpt',
    label: 'ChatGPT',
    tabs: ['chatgpt', 'codex'],
    seoTitle: 'Publish pages from ChatGPT with the Canvasbin MCP app',
    seoDescription:
      'Connect Canvasbin to ChatGPT and turn ChatGPT’s answers, HTML and canvas drafts into shareable web pages and PDFs that ChatGPT can keep up to date.',
    title: 'Publish pages from ChatGPT',
    lead: 'Add Canvasbin to ChatGPT as an app. ChatGPT can then publish its answers, HTML and canvas drafts as pages with a shareable link and a PDF, and update them later.',
    prompts: [
      'Make this into a nicely formatted Canvasbin page and send me the link.',
      'Publish this meeting summary as a page I can share with the client.',
      'Turn this table into an interactive chart page on Canvasbin.',
      'Update the itinerary page with the new flight times.',
    ],
    faqs: [
      [
        'Can I share a ChatGPT canvas as a web page?',
        'Yes. Ask ChatGPT to publish the canvas to Canvasbin and you get a link anyone can open, plus a PDF download. Ask again later and ChatGPT updates the same page.',
      ],
      [
        'Who can see the pages ChatGPT publishes?',
        'Only people with the link, or people you invite. Pages have unguessable addresses and are kept out of search engines.',
      ],
      [
        'Does it work with Codex?',
        'Yes. Add Canvasbin to Codex with one command, sign in, and Codex can publish pages from your terminal.',
      ],
    ],
  },
  {
    slug: 'cursor',
    label: 'Cursor',
    tabs: ['cursor', 'vscode'],
    seoTitle: 'Canvasbin MCP for Cursor: publish dashboards, docs and reports',
    seoDescription:
      'Install the Canvasbin MCP server in Cursor with one click and let Cursor’s agent publish release notes, dashboards and architecture diagrams as shareable pages.',
    title: 'Publish pages from Cursor',
    lead: 'Install Canvasbin in Cursor with one click. Cursor’s agent can then publish release notes, dashboards and diagrams from your codebase as pages with a shareable link, and refresh them on a schedule with Cursor Automations.',
    prompts: [
      'Write release notes for the last 20 commits and publish them on Canvasbin.',
      'Draw an architecture diagram of this repo as a Canvasbin page.',
      'Publish a test coverage dashboard and update it after each run.',
      'Every Monday, update the engineering weekly page from merged pull requests.',
    ],
    faqs: [
      [
        'Can Cursor keep a dashboard up to date automatically?',
        'Yes. Run the same prompt from a Cursor Automation on a schedule. The agent updates the page in place, so the link you shared never changes.',
      ],
      [
        'Does it work in VS Code too?',
        'Yes. Install Canvasbin in VS Code with one click and use it from GitHub Copilot agent mode.',
      ],
      [
        'Can the agent publish full HTML with scripts and charts?',
        'Yes. Pages can be complete HTML documents with their own styles, scripts and chart libraries. Scripts run in a sandbox, isolated from your account.',
      ],
    ],
  },
]

// What an assistant can do once connected; mirrors the MCP tools.
export const connectTools = [
  ['Publish pages', 'Create an HTML or Markdown page and get its shareable link, PDF link and edit link.', 'create_page'],
  ['Update pages', 'Change a page’s content or title. Everyone viewing it sees the change live.', 'update_page'],
  ['Read pages', 'Fetch a page back to revise, summarise or build on it.', 'get_page'],
  ['Browse your library', 'List your pages, pages shared with you and your team pages.', 'list_pages'],
  ['Delete pages', 'Remove a page you no longer need. Clients that support it ask you to confirm first.', 'delete_page'],
  ['Share with people', 'Invite people by email as viewers or editors (Team plan).', 'share_page'],
  ['Publish to a team', 'Create pages in a shared team space everyone can edit.', 'list_teams'],
  ['Comment', 'Read and reply to comments on team pages.', 'add_comment'],
  [
    'Follow a style guide',
    'On the Team plan, read how a team wants its pages written before creating one.',
    'get_style_guide',
  ],
  [
    'Set a style guide',
    'On the Team plan, create or update how a team wants its pages written. Any member can do this.',
    'set_style_guide',
  ],
]

export const findAssistant = (slug) => assistants.find((a) => a.slug === slug)
