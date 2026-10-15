// describeActivity turns a timeline entry into a sentence (without the time).
export function describeActivity(a) {
  const who = a.user_name || a.user_email || 'Someone'
  const page = a.page_title || 'Untitled'
  switch (a.action) {
    case 'page_created':
      return `${who} created “${page}”`
    case 'page_edited':
      return `${who} edited “${page}”`
    case 'page_deleted':
      return `${who} deleted “${page}”`
    case 'comment_added':
      return `${who} commented on “${page}”`
    case 'member_joined':
      return `${who} joined the team${a.detail ? ` (${a.detail})` : ''}`
    case 'member_invited':
      return `${who} invited ${a.detail}`
    case 'member_removed':
      return `${who} removed ${a.detail}`
    case 'member_left':
      return `${who} left the team`
    case 'role_changed':
      return `${who} changed a role: ${a.detail}`
    case 'team_renamed':
      return `${who} renamed the team to “${a.detail}”`
    default:
      return `${who}: ${a.action}`
  }
}

export const formatWhen = (s) => (s ? new Date(s).toLocaleString() : '')
