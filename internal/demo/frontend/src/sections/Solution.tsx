const NODES = [
  { name: 'Recall', desc: 'Check known failures', color: 'bg-rh-teal-10 border-rh-teal-50' },
  { name: 'Triage', desc: 'Classify symptoms', color: 'bg-rh-teal-10 border-rh-teal-50' },
  { name: 'Resolve', desc: 'Identify repos', color: 'bg-rh-purple-50/10 border-rh-purple-50' },
  { name: 'Investigate', desc: 'Gather evidence', color: 'bg-rh-purple-50/10 border-rh-purple-50' },
  { name: 'Correlate', desc: 'Cross-reference', color: 'bg-rh-red-10 border-rh-red-50' },
  { name: 'Review', desc: 'Validate findings', color: 'bg-rh-red-10 border-rh-red-50' },
  { name: 'Report', desc: 'File RCA report', color: 'bg-rh-gray-20/50 border-rh-gray-60' },
]

export function Solution() {
  return (
    <section id="solution" data-kami="section:solution" className="section flex items-center justify-center bg-white px-8" aria-label="The Solution">
      <div className="max-w-5xl text-center">
        <h2 className="text-4xl font-bold text-rh-gray-80 mb-4">The Solution</h2>
        <p className="text-rh-gray-60 text-lg mb-10 max-w-2xl mx-auto">
          A 7-node AI pipeline that <strong>recalls</strong> known failures,{' '}
          <strong>triages</strong> symptoms, <strong>investigates</strong> evidence,{' '}
          and <strong>reports</strong> root causes — with confidence scores.
        </p>
        <div className="flex flex-wrap justify-center gap-4">
          {NODES.map((node, i) => (
            <div key={node.name} className="flex items-center gap-3">
              <div data-kami={`node:${node.name.toLowerCase()}`} className={`rounded-xl border-2 px-5 py-3 ${node.color}`}>
                <div className="font-bold text-rh-gray-80">{node.name}</div>
                <div className="text-xs text-rh-gray-60">{node.desc}</div>
              </div>
              {i < NODES.length - 1 && (
                <span className="text-rh-gray-40 text-xl">→</span>
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
