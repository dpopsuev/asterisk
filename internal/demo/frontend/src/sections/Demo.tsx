export function Demo() {
  return (
    <section id="demo" data-kami="section:demo" className="section flex flex-col items-center justify-center bg-white px-8" aria-label="Live Demo">
      <div className="w-full max-w-6xl">
        <h2 className="text-4xl font-bold text-rh-gray-80 mb-4 text-center">Live Demo</h2>
        <p className="text-center text-rh-gray-60 mb-8">
          Watch the AI agents investigate a real CI failure in real-time.
        </p>
        <div className="w-full aspect-video rounded-2xl border-2 border-rh-gray-20 bg-rh-gray-80 flex items-center justify-center">
          <div className="text-center text-rh-gray-40">
            <p className="text-lg mb-2">Kami Live Debugger</p>
            <p className="text-sm">SSE event stream connects here when running with <code className="bg-rh-gray-60 text-white px-2 py-0.5 rounded">--live</code> or <code className="bg-rh-gray-60 text-white px-2 py-0.5 rounded">--replay</code></p>
          </div>
        </div>
      </div>
    </section>
  )
}
