export function Hero() {
  return (
    <section id="hero" data-kami="section:hero" className="section flex flex-col items-center justify-center bg-rh-gray-80 text-white px-8" aria-label="Introduction">
      <div className="text-center max-w-3xl">
        <div className="text-6xl font-black tracking-tight mb-4">
          <span className="text-rh-red-50">Asterisk</span>
        </div>
        <p className="text-2xl font-light text-rh-gray-20 mb-8">
          AI-Driven Root-Cause Analysis for CI Failures
        </p>
        <p className="text-rh-teal-30 text-lg mb-2">
          Powered by <span className="font-semibold text-rh-purple-50">Origami</span> — graph-based agentic pipeline framework
        </p>
        <div className="mt-12 text-rh-gray-40 text-sm animate-bounce">
          ↓ Scroll to explore
        </div>
      </div>
    </section>
  )
}
