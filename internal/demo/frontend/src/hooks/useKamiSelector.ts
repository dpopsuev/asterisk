import { useEffect, useRef, useCallback } from 'react'

export interface SelectedElement {
  type: string
  id: string
  kamiKey: string
}

export interface KamiSelection {
  elements: SelectedElement[]
  timestamp: string
}

/**
 * useKamiSelector enables CTRL+hover/click element selection for AI debugging.
 *
 * - CTRL + hover: soft blinking white highlight (transient)
 * - CTRL + click: purple sparkling highlight (persisted, toggle)
 * - CTRL + click parent: consumes selected children via DOM containment
 *
 * Elements are identified by `data-kami="<type>:<id>"` attributes.
 * Selection payloads are POSTed to `/events/selection` for MCP tool retrieval.
 */
export function useKamiSelector(enabled: boolean) {
  const selectedRef = useRef<Map<string, HTMLElement>>(new Map())
  const hoveredRef = useRef<HTMLElement | null>(null)

  const postSelection = useCallback(() => {
    const elements: SelectedElement[] = []
    for (const [key, el] of selectedRef.current) {
      const attr = el.getAttribute('data-kami') ?? key
      const [type, id] = attr.split(':', 2)
      elements.push({ type, id, kamiKey: key })
    }
    const payload: KamiSelection = {
      elements,
      timestamp: new Date().toISOString(),
    }

    // Expose on bridge
    if (window.__origami) {
      (window.__origami as Record<string, unknown>).selection = payload
    }

    // POST to Kami server (fire-and-forget)
    fetch('/events/selection', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    }).catch(() => {})
  }, [])

  const clearHover = useCallback(() => {
    if (hoveredRef.current) {
      hoveredRef.current.classList.remove('kami-hover')
      hoveredRef.current = null
    }
  }, [])

  useEffect(() => {
    if (!enabled) return

    function findKamiParent(target: EventTarget | null): HTMLElement | null {
      let el = target as HTMLElement | null
      while (el) {
        if (el.hasAttribute?.('data-kami')) return el
        el = el.parentElement
      }
      return null
    }

    function handleMouseOver(e: MouseEvent) {
      if (!e.ctrlKey && !e.metaKey) {
        clearHover()
        return
      }
      const kamiEl = findKamiParent(e.target)
      if (kamiEl === hoveredRef.current) return

      clearHover()
      if (kamiEl) {
        kamiEl.classList.add('kami-hover')
        hoveredRef.current = kamiEl
      }
    }

    function handleMouseOut(e: MouseEvent) {
      const kamiEl = findKamiParent(e.target)
      if (kamiEl && kamiEl === hoveredRef.current) {
        clearHover()
      }
    }

    function handleKeyUp(e: KeyboardEvent) {
      if (e.key === 'Control' || e.key === 'Meta') {
        clearHover()
      }
    }

    function handleClick(e: MouseEvent) {
      if (!e.ctrlKey && !e.metaKey) return
      const kamiEl = findKamiParent(e.target)
      if (!kamiEl) return

      e.preventDefault()
      e.stopPropagation()

      const key = kamiEl.getAttribute('data-kami')!
      const selected = selectedRef.current

      if (selected.has(key)) {
        // Deselect
        selected.delete(key)
        kamiEl.classList.remove('kami-selected')
      } else {
        // Parent-child consumption: remove any descendants
        for (const [existingKey, existingEl] of selected) {
          if (kamiEl.contains(existingEl) && existingKey !== key) {
            selected.delete(existingKey)
            existingEl.classList.remove('kami-selected')
          }
        }
        selected.set(key, kamiEl)
        kamiEl.classList.add('kami-selected')
      }

      postSelection()
    }

    document.addEventListener('mouseover', handleMouseOver, true)
    document.addEventListener('mouseout', handleMouseOut, true)
    document.addEventListener('click', handleClick, true)
    document.addEventListener('keyup', handleKeyUp, true)

    return () => {
      document.removeEventListener('mouseover', handleMouseOver, true)
      document.removeEventListener('mouseout', handleMouseOut, true)
      document.removeEventListener('click', handleClick, true)
      document.removeEventListener('keyup', handleKeyUp, true)

      // Cleanup visual state
      clearHover()
      for (const [, el] of selectedRef.current) {
        el.classList.remove('kami-selected')
      }
      selectedRef.current.clear()
    }
  }, [enabled, clearHover, postSelection])
}
