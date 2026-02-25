const METRICS = [
  { label: 'BasicAdapter M19', value: 0.83, color: 'bg-rh-teal-50' },
  { label: 'CursorAdapter M19', value: 0.58, color: 'bg-rh-purple-50' },
]

export function Results() {
  return (
    <section id="results" data-kami="section:results" className="section flex items-center justify-center bg-rh-gray-80 text-white px-8" aria-label="Results">
      <div className="max-w-4xl w-full">
        <h2 className="text-4xl font-bold mb-8 text-center">PoC Results</h2>
        <p className="text-center text-rh-gray-40 mb-10">
          Calibrated on 18 verified PTP Operator test failures
        </p>
        <div className="grid md:grid-cols-2 gap-8">
          {METRICS.map((m) => (
            <div key={m.label} data-kami={`metric:${m.label.replace(/\s+/g, '-').toLowerCase()}`} className="bg-rh-gray-60/30 rounded-2xl p-6">
              <div className="text-rh-gray-40 text-sm mb-2">{m.label}</div>
              <div className="text-5xl font-black mb-4">{m.value.toFixed(2)}</div>
              <div className="w-full bg-rh-gray-60 rounded-full h-3">
                <div
                  className={`${m.color} h-3 rounded-full transition-all duration-1000`}
                  style={{ width: `${m.value * 100}%` }}
                />
              </div>
            </div>
          ))}
        </div>
        <div className="mt-8 grid grid-cols-3 gap-4 text-center">
          <div className="bg-rh-gray-60/30 rounded-xl p-4">
            <div className="text-2xl font-bold text-rh-teal-30">19/21</div>
            <div className="text-xs text-rh-gray-40">Metrics passing (Basic)</div>
          </div>
          <div className="bg-rh-gray-60/30 rounded-xl p-4">
            <div className="text-2xl font-bold text-rh-purple-50">18</div>
            <div className="text-xs text-rh-gray-40">Verified cases</div>
          </div>
          <div className="bg-rh-gray-60/30 rounded-xl p-4">
            <div className="text-2xl font-bold text-rh-red-40">7</div>
            <div className="text-xs text-rh-gray-40">Pipeline nodes</div>
          </div>
        </div>
      </div>
    </section>
  )
}
