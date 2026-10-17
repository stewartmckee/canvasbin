// Content for the /use-cases landing pages. Sample keys are read-only links to
// live pages kept up to date by an agent through the Canvasbin MCP.

export const sampleUrl = (key) => `https://canvasbin.dev/${key}.html`

export const useCases = [
  {
    slug: 'developers',
    label: 'Developers',
    seoTitle: 'AI-built dashboards and diagrams for developers',
    seoDescription:
      'Release health dashboards, architecture diagrams and weekly engineering digests that your AI agent builds from your repos and keeps up to date at one link.',
    icon: '⚙️',
    eyebrow: 'For developers and engineering teams',
    title: 'Dashboards, diagrams and digests your agent keeps up to date.',
    lead:
      'Ask Cursor, Claude or ChatGPT to turn CI runs, pull requests, traces and incidents into a real web page with charts and diagrams. Put it on a schedule and the same link always shows the latest picture.',
    summary: 'Release health, architecture maps and weekly engineering digests, generated from your repos and tools.',
    samples: [
      {
        key: 'ighVznEhEgsyV_SWIhPOWHKSNDQAcFjD3GUad1nQjvA',
        title: 'Release health dashboard',
        body: 'DORA metrics, a live pipeline view, CI trends and flaky tests, built with Chart.js.',
        schedule: 'Refreshed every 30 minutes from GitHub Actions, Sentry and Datadog',
      },
      {
        key: 'SgE-xuRl-V_K5qWs6dZflLY0_HsPB0mJKETKxmqFhVQ',
        title: 'Architecture map',
        body: 'System, sequence and data model diagrams in Mermaid, plus what changed since last week.',
        schedule: 'Regenerated nightly from the monorepo and Terraform',
      },
      {
        key: 'iAwgzpisUr2gBzk7crc6stihpqZ6lkl117xGG-WCiwI',
        title: 'Engineering weekly',
        body: 'A readable Monday digest of merged PRs, incidents and what is coming next.',
        schedule: 'Written every Monday at 07:00',
      },
    ],
    prompts: [
      'Build a release health page for this repo from the last 14 days of GitHub Actions runs and publish it to Canvasbin.',
      'Draw our service architecture as Mermaid diagrams from the code and Terraform, and update the Canvasbin page every night.',
      'Every Monday at 7am, summarise last week’s merged PRs and incidents into our engineering weekly page.',
      'Turn this incident timeline into a postmortem page with a chart of error rates and share it with the team.',
    ],
    ideas: [
      ['On-call handover', 'Open alerts, recent pages and runbook links, refreshed at every shift change.'],
      ['API status board', 'Latency and error rates per endpoint, pulled from your metrics every few minutes.'],
      ['Release notes', 'Customer-facing notes written from the changelog for each deploy.'],
      ['Dependency report', 'Outdated and vulnerable packages across every repo, ranked by risk.'],
      ['Benchmark results', 'Performance runs charted over time so regressions stand out.'],
      ['PR review queue', 'Who is waiting on whom, sorted by age, posted every morning.'],
    ],
  },
  {
    slug: 'business',
    label: 'Business',
    seoTitle: 'Self-updating reports and dashboards for business',
    seoDescription:
      'Sales pipelines, support dashboards and client status pages that your AI agent rebuilds from your CRM, helpdesk and project tools on a schedule.',
    icon: '📈',
    eyebrow: 'For founders, operators and client teams',
    title: 'Reports that build themselves and never go stale.',
    lead:
      'Your agent pulls numbers from your CRM, helpdesk and project tools, then publishes a polished, interactive page. Share one link with your team or your client, and it updates itself on a schedule.',
    summary: 'Sales pipelines, support pulses and client status pages, refreshed from your CRM, helpdesk and project tools.',
    samples: [
      {
        key: 'ch54yZy_tjz9hgDg6kzdU8_oj7vQT0Z2lPfCtvzLcvo',
        title: 'Sales pipeline',
        body: 'Quota attainment, bookings against target, an interactive funnel and a region filter.',
        schedule: 'Pulled from HubSpot every morning at 06:30',
      },
      {
        key: '0MUAWlAGO7jOB8SF1SyFVDWQwvVO9K5kXySmd6knflI',
        title: 'Customer support pulse',
        body: 'CSAT, ticket volume, top contact reasons and what customers actually said.',
        schedule: 'Summarised from Zendesk every morning',
      },
      {
        key: 'twmFyzkRA5YPXYpjsTYwZQUAel2OwDbPzwiCl17NlMY',
        title: 'Client project status',
        body: 'RAG status, a live timeline, milestones, budget and what you need from the client.',
        schedule: 'Compiled every Friday from Linear and Harvest',
      },
    ],
    prompts: [
      'Every weekday at 6:30, build a sales pipeline page from HubSpot with bookings against target and the deals to watch.',
      'Summarise this week’s support tickets into a pulse page with CSAT, top reasons and three customer quotes.',
      'Each Friday, update the Riverside project status page from Linear and Harvest, then share it with the client.',
      'Turn this spreadsheet into a board update page with charts, and keep it in our team space.',
    ],
    ideas: [
      ['Board and investor updates', 'Monthly metrics and narrative in one link, with charts that are always current.'],
      ['Marketing performance', 'Traffic, signups and campaign spend from your analytics tools.'],
      ['Hiring tracker', 'Open roles, pipeline by stage and interviews this week.'],
      ['Finance snapshot', 'Cash, burn and runway from your accounting software, refreshed monthly.'],
      ['Meeting briefs', 'An agenda page with the latest numbers, generated an hour before each meeting.'],
      ['Client portals', 'One read-only link per client with deliverables, status and invoices.'],
    ],
  },
  {
    slug: 'family',
    label: 'Family & hobbies',
    seoTitle: 'Family dashboards your AI keeps up to date',
    seoDescription:
      'A junior parkrun dashboard rebuilt from result emails every Sunday, plus ideas for club tables, holiday planners and chore boards your AI keeps current.',
    icon: '🏃',
    eyebrow: 'For families, clubs and hobbies',
    title: 'A family dashboard that updates itself every Sunday.',
    lead:
      'Our kids run junior parkrun every Sunday. An agent reads the result emails, works out new personal bests and milestones, and updates one page the whole family can open on their phones.',
    summary: 'Our kids’ junior parkrun dashboard, rebuilt from result emails every Sunday, plus ideas for clubs and hobbies.',
    samples: [
      {
        key: '7yCQkLxLYG-z0RWkwCnJSt-2NMACbhowgUu-4qFtSFI',
        title: 'McKee junior parkrun HQ',
        body: 'Run charts with PB stars, a wristband milestone wall, head to head stats and a Sunday sticker chart.',
        schedule: 'Rebuilt from parkrun result emails every Sunday afternoon',
        featured: true,
      },
    ],
    prompts: [
      'Read our parkrun result emails and build a family dashboard with each child’s times, PBs and milestones.',
      'Every Sunday at 2pm, check for new parkrun results and update the dashboard. Celebrate any new PB at the top.',
      'Add a sticker chart of the Sundays we all ran together, and make it look great on a phone.',
    ],
    ideas: [
      ['Club league tables', 'Results and standings for a running, football or chess club, after every fixture.'],
      ['Holiday planner', 'Itinerary, bookings, maps and a countdown, kept in sync with your email.'],
      ['Reading challenge', 'Books read by each child this year, with a progress chart.'],
      ['Garden and allotment log', 'Weather, sowing dates and harvests, updated weekly.'],
      ['Family chores board', 'Who did what this week, with points and a leaderboard.'],
      ['Fitness tracker', 'Weekly runs, rides or swims from your watch, charted over the year.'],
    ],
  },
]

export const findUseCase = (slug) => useCases.find((u) => u.slug === slug)

export const capabilities = [
  ['Charts and graphs', 'Chart.js, D3, ECharts or hand-drawn SVG, whatever the agent likes.'],
  ['Diagrams', 'Mermaid flowcharts, sequence diagrams and data models.'],
  ['Interactivity', 'Filters, tabs, tooltips and toggles, all in plain JavaScript.'],
  ['Any layout', 'Full HTML and CSS: dark mode, animation, custom fonts, mobile-first, plus a one-click PDF.'],
  ['Live updates', 'When the agent updates a page, open browsers refresh by themselves. Great for wall screens.'],
  ['Safe by default', 'Scripts run in a sandbox, away from Canvasbin and your account.'],
]

export const schedulers = [
  ['Cursor Automations', 'Run an agent on a schedule or trigger that publishes with the Canvasbin MCP.'],
  ['Claude and ChatGPT tasks', 'Scheduled tasks that ask your assistant to refresh the page.'],
  ['Cron or CI', 'Any job that calls an agent, or the Canvasbin API, with your page’s edit key.'],
]
