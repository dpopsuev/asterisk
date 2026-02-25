export function Problem() {
  return (
    <section id="problem" data-kami="section:problem" className="section flex items-center justify-center bg-rh-gray-80 text-white px-8" aria-label="The Problem">
      <div className="max-w-4xl grid md:grid-cols-2 gap-12 items-center">
        <div>
          <h2 className="text-4xl font-bold mb-6">The Problem</h2>
          <p className="text-rh-gray-20 text-lg leading-relaxed mb-4">
            CI failures are inevitable. Understanding <em>why</em> they fail is the bottleneck.
          </p>
          <ul className="text-rh-gray-20 space-y-3">
            <li className="flex items-start gap-2">
              <span className="text-rh-red-50 mt-1">●</span>
              Engineers spend hours triaging failures manually
            </li>
            <li className="flex items-start gap-2">
              <span className="text-rh-red-50 mt-1">●</span>
              Root causes are scattered across logs, commits, and infra
            </li>
            <li className="flex items-start gap-2">
              <span className="text-rh-red-50 mt-1">●</span>
              Known failures recur without institutional memory
            </li>
            <li className="flex items-start gap-2">
              <span className="text-rh-red-50 mt-1">●</span>
              "It works on my machine" is not root-cause analysis
            </li>
          </ul>
        </div>
        <div className="flex flex-col items-center gap-4">
          <div className="text-7xl font-black text-rh-red-50">60%</div>
          <p className="text-rh-gray-40 text-center">
            of QE time spent on manual failure triage
          </p>
        </div>
      </div>
    </section>
  )
}
