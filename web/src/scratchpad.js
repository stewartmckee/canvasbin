// Marketing content for /agent-scratchpad/* landing pages.

export const scratchpadHub = {
  seoTitle: 'Agent scratchpads: external storage for your AI',
  seoDescription:
    'Back up agent context and share project notes across every assistant you connect. Agent scratchpads stay out of your web library by default but are always available over MCP.',
}

export const scratchpadPages = [
  {
    slug: 'share-context-across-platforms',
    label: 'Share context across platforms',
    seoTitle: 'Share AI context across platforms',
    seoDescription:
      'Keep one agent scratchpad on Canvasbin that every MCP-connected assistant can read and update, so you do not re-brief each platform from scratch.',
    eyebrow: 'Cross-platform context',
    title: 'One context sheet every assistant can read.',
    lead:
      'Your assistants do not share memory. Store project context, decisions and open questions in an agentic-only Canvasbin page and let whichever tool you use pick up where you left off.',
    problem: [
      'Each assistant keeps its own threads and forgets what you told another.',
      'Switching tools means repeating stack choices, names, and constraints.',
      'Pasting the same briefing into every chat wastes time and tokens.',
    ],
    solution: [
      'Ask an agent to create or update a Canvasbin page with agentic_only set.',
      'The page lives in your account but stays hidden from the web library by default.',
      'Any MCP client signed in to your account can list_pages, get_page, create_draft and publish_draft.',
    ],
    prompts: [
      'Create an agent scratchpad on Canvasbin with my project context (stack, decisions, open questions). Set agentic_only true.',
      'Read my Canvasbin context scratchpad and continue from it in this chat.',
      'Update my Canvasbin scratchpad with what we decided in this thread.',
    ],
    faqs: [
      [
        'Is this different from a normal Canvasbin page?',
        'Yes. Agent scratchpads are marked agentic_only. They work like any other page over MCP, but the web library hides them unless you turn on “Show agent scratchpads”.',
      ],
      [
        'Can I still open the page in a browser?',
        'Yes. You get the same read and write links. Scratchpads are for storage and continuity, not polished sharing.',
      ],
    ],
  },
  {
    slug: 'backup-ai-chats',
    label: 'Backup AI chats',
    seoTitle: 'Back up AI chats you own on Canvasbin',
    seoDescription:
      'Back up agent context and chat threads on Canvasbin agent scratchpads — durable storage outside any single chat provider.',
    eyebrow: 'Chat backup',
    title: 'A backup you control when the chat app cannot help.',
    lead:
      'Token limits, lost threads, account issues, or switching providers can strand important work inside one product. Store summaries and exports in an agentic-only page you can update in place.',
    problem: [
      'Long threads fall off context windows or disappear when a session ends.',
      'You may not be able to export or search old conversations everywhere you work.',
      'Relying on one vendor means lock-in for the notes that matter.',
    ],
    solution: [
      'Ask your agent to append or replace an agentic-only scratchpad with structured notes: date, decisions, todos, key quotes.',
      'The same link always shows the latest version after publish_draft.',
      'Data sits in your Canvasbin library logic over MCP, not only inside the chat UI.',
    ],
    prompts: [
      'Back up the important parts of this conversation to my Canvasbin agent scratchpad and update it in place.',
      'Every Friday, refresh my chat-backup scratchpad with this week’s decisions and open todos.',
      'Summarise this thread into Markdown on my agent scratchpad with agentic_only true.',
    ],
    faqs: [
      [
        'Does Canvasbin mirror my full chat transcript automatically?',
        'No. Your assistant writes what you ask it to store — usually a summary or an export you paste. You choose what to keep.',
      ],
      [
        'Where do scratchpads show up?',
        'In list_pages for your agent. On the website they are hidden until you enable “Show agent scratchpads” on the Pages screen.',
      ],
    ],
  },
]

export function findScratchpad(slug) {
  return scratchpadPages.find((p) => p.slug === slug)
}
