import { fireEvent, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { KeyAction } from '../calculator/types'
import { useKeyboard } from './useKeyboard'

function setup() {
  const onAction = vi.fn<(action: KeyAction) => void>()
  const hook = renderHook(() => useKeyboard(onAction))
  return { onAction, ...hook }
}

describe('useKeyboard', () => {
  it('dispatches the mapped action and prevents the default', () => {
    const { onAction } = setup()

    const notCancelled = fireEvent.keyDown(window, { key: '7' })

    expect(onAction).toHaveBeenCalledWith({ type: 'DIGIT', digit: '7' })
    expect(notCancelled).toBe(false)
  })

  it.each([
    ['an unmapped key', { key: 'a' }],
    ['a browser shortcut', { key: 'r', ctrlKey: true }],
  ])('leaves %s alone', (_, init) => {
    const { onAction } = setup()

    const notCancelled = fireEvent.keyDown(window, init)

    expect(onAction).not.toHaveBeenCalled()
    expect(notCancelled).toBe(true)
  })

  it('stops listening when unmounted', () => {
    const { onAction, unmount } = setup()

    unmount()
    fireEvent.keyDown(window, { key: '7' })

    expect(onAction).not.toHaveBeenCalled()
  })
})
