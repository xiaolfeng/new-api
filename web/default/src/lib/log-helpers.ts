export function getUserRole(): number {
  const raw = localStorage.getItem('user')
  if (!raw) return 0
  try {
    const parsed = JSON.parse(raw)
    return typeof parsed.role === 'number' ? parsed.role : 0
  } catch {
    return 0
  }
}

export function hasDeveloperToolLogAccess(): boolean {
  return getUserRole() >= 2
}
