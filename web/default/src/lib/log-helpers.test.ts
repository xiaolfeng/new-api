import { describe, it, expect, beforeEach, afterEach } from 'bun:test'
import { getUserRole, hasDeveloperToolLogAccess } from './log-helpers'

const mockLocalStorage = {
  _data: {} as Record<string, string>,
  getItem(key: string) {
    return this._data[key] ?? null
  },
  setItem(key: string, value: string) {
    this._data[key] = value
  },
  removeItem(key: string) {
    delete this._data[key]
  },
  clear() {
    this._data = {}
  },
}

;(globalThis as unknown as { localStorage: typeof mockLocalStorage }).localStorage = mockLocalStorage

describe('getUserRole', () => {
  beforeEach(() => {
    mockLocalStorage.clear()
  })

  it('returns 0 when no user in localStorage', () => {
    expect(getUserRole()).toBe(0)
  })

  it('returns role from user object', () => {
    mockLocalStorage.setItem('user', JSON.stringify({ role: 10 }))
    expect(getUserRole()).toBe(10)
  })

  it('returns 0 for non-number role', () => {
    mockLocalStorage.setItem('user', JSON.stringify({ role: 'admin' }))
    expect(getUserRole()).toBe(0)
  })

  it('returns 0 for malformed JSON', () => {
    mockLocalStorage.setItem('user', 'not-json')
    expect(getUserRole()).toBe(0)
  })

  it('returns correct role for code user (2)', () => {
    mockLocalStorage.setItem('user', JSON.stringify({ role: 2 }))
    expect(getUserRole()).toBe(2)
  })
})

describe('hasDeveloperToolLogAccess', () => {
  beforeEach(() => {
    mockLocalStorage.clear()
  })

  it('returns false for guest (role 0)', () => {
    expect(hasDeveloperToolLogAccess()).toBe(false)
  })

  it('returns false for regular user (role 1)', () => {
    mockLocalStorage.setItem('user', JSON.stringify({ role: 1 }))
    expect(hasDeveloperToolLogAccess()).toBe(false)
  })

  it('returns true for code user (role 2)', () => {
    mockLocalStorage.setItem('user', JSON.stringify({ role: 2 }))
    expect(hasDeveloperToolLogAccess()).toBe(true)
  })

  it('returns true for admin (role 10)', () => {
    mockLocalStorage.setItem('user', JSON.stringify({ role: 10 }))
    expect(hasDeveloperToolLogAccess()).toBe(true)
  })
})
