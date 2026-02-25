const AGENTS = [
  {
    name: 'Herald',
    element: 'Fire',
    role: 'Lead Detective',
    line: '"I saw the error. I already know what happened. You\'re welcome."',
    color: 'border-rh-red-50 bg-rh-red-10',
    badge: '🔥',
  },
  {
    name: 'Seeker',
    element: 'Water',
    role: 'Forensic Analyst',
    line: '"Let\'s not jump to conclusions. I\'d like to examine all 47 log files first."',
    color: 'border-rh-teal-50 bg-rh-teal-10',
    badge: '🌊',
  },
  {
    name: 'Sentinel',
    element: 'Earth',
    role: 'Desk Sergeant',
    line: '"I\'ve filed this under \'infrastructure.\' Next case."',
    color: 'border-rh-purple-50 bg-rh-purple-50/10',
    badge: '🪨',
  },
  {
    name: 'Weaver',
    element: 'Air',
    role: 'Undercover Agent',
    line: '"What if the bug isn\'t in the code? What if it\'s in the process?"',
    color: 'border-rh-teal-30 bg-rh-teal-10',
    badge: '💨',
  },
  {
    name: 'Arbiter',
    element: 'Diamond',
    role: 'Internal Affairs',
    line: '"The evidence is inconclusive. I\'m reopening the investigation."',
    color: 'border-rh-gray-60 bg-rh-gray-20/50',
    badge: '💎',
  },
  {
    name: 'Catalyst',
    element: 'Lightning',
    role: 'Dispatch',
    line: '"New failure incoming! All units respond!"',
    color: 'border-rh-red-40 bg-rh-red-10',
    badge: '⚡',
  },
]

export function Agents() {
  return (
    <section id="agents" data-kami="section:agents" className="section flex items-center justify-center bg-rh-gray-80 text-white px-8" aria-label="Meet the Agents">
      <div className="max-w-5xl">
        <h2 className="text-4xl font-bold mb-8 text-center">Meet the Detectives</h2>
        <p className="text-center text-rh-gray-40 mb-10">
          Each AI agent has a distinct personality, element affinity, and investigative style.
        </p>
        <div className="grid md:grid-cols-3 gap-5">
          {AGENTS.map((a) => (
            <div
              key={a.name}
              data-kami={`agent:${a.name}`}
              className={`rounded-2xl border-2 p-5 ${a.color} text-rh-gray-80`}
            >
              <div className="flex items-center gap-2 mb-2">
                <span className="text-2xl">{a.badge}</span>
                <div>
                  <div className="font-bold text-lg">{a.name}</div>
                  <div className="text-xs text-rh-gray-60">{a.role} · {a.element}</div>
                </div>
              </div>
              <p className="text-sm italic text-rh-gray-60">{a.line}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
