import { describe, it, expect, beforeEach } from 'vitest'

import { getUserRole, hasDeveloperToolLogAccess } from './log-helpers'

describe('getUserRole', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns 0 when no user in localStorage', () => {
    expect(getUserRole()).toBe(0)
  })

  it('returns role from user object', () => {
    localStorage.setItem('user', JSON.stringify({ role: 10 }))
    expect(getUserRole()).toBe(10)
  })

  it('returns 0 for non-number role', () => {
    localStorage.setItem('user', JSON.stringify({ role: 'admin' }))
    expect(getUserRole()).toBe(0)
  })

  it('returns 0 for malformed JSON', () => {
    localStorage.setItem('user', 'not-json')
    expect(getUserRole()).toBe(0)
  })

  it('returns correct role for code user (2)', () => {
    localStorage.setItem('user', JSON.stringify({ role: 2 }))
    expect(getUserRole()).toBe(2)
  })
})

describe('hasDeveloperToolLogAccess', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns false for guest (role 0)', () => {
    expect(hasDeveloperToolLogAccess()).toBe(false)
  })

  it('returns false for regular user (role 1)', () => {
    localStorage.setItem('user', JSON.stringify({ role: 1 }))
    expect(hasDeveloperToolLogAccess()).toBe(false)
  })

  it('returns true for code user (role 2)', () => {
    localStorage.setItem('user', JSON.stringify({ role: 2 }))
    expect(hasDeveloperToolLogAccess()).toBe(true)
  })

  it('returns true for admin (role 10)', () => {
    localStorage.setItem('user', JSON.stringify({ role: 10 }))
    expect(hasDeveloperToolLogAccess()).toBe(true)
  })
})
