import { useEffect, useState } from 'react'
import { useKamiSelector } from './hooks/useKamiSelector'
import './selector.css'
import { Hero } from './sections/Hero'
import { Agenda } from './sections/Agenda'
import { Problem } from './sections/Problem'
import { Solution } from './sections/Solution'
import { Agents } from './sections/Agents'
import { Transition } from './sections/Transition'
import { Demo } from './sections/Demo'
import { Results } from './sections/Results'
import { Competitive } from './sections/Competitive'
import { Architecture } from './sections/Architecture'
import { Roadmap } from './sections/Roadmap'
import { Closing } from './sections/Closing'

const SECTION_IDS = [
  'hero', 'agenda', 'problem', 'solution', 'agents',
  'transition', 'demo', 'results', 'competitive',
  'architecture', 'roadmap', 'closing',
]

function App() {
  const [activeSection, setActiveSection] = useState('hero')
  useKamiSelector(true)

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id)
          }
        }
      },
      { threshold: 0.5 }
    )

    for (const id of SECTION_IDS) {
      const el = document.getElementById(id)
      if (el) observer.observe(el)
    }

    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      const idx = SECTION_IDS.indexOf(activeSection)
      if (e.key === 'ArrowDown' || e.key === 'PageDown') {
        e.preventDefault()
        const next = Math.min(idx + 1, SECTION_IDS.length - 1)
        document.getElementById(SECTION_IDS[next])?.scrollIntoView({ behavior: 'smooth' })
      } else if (e.key === 'ArrowUp' || e.key === 'PageUp') {
        e.preventDefault()
        const prev = Math.max(idx - 1, 0)
        document.getElementById(SECTION_IDS[prev])?.scrollIntoView({ behavior: 'smooth' })
      }
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [activeSection])

  return (
    <main>
      <Hero />
      <Agenda activeSection={activeSection} />
      <Problem />
      <Solution />
      <Agents />
      <Transition />
      <Demo />
      <Results />
      <Competitive />
      <Architecture />
      <Roadmap />
      <Closing />
    </main>
  )
}

export default App
