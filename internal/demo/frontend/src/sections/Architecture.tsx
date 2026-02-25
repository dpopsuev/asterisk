export function Architecture() {
  return (
    <section id="architecture" data-kami="section:architecture" className="section flex items-center justify-center bg-rh-gray-80 text-white px-8" aria-label="Architecture">
      <div className="max-w-4xl text-center">
        <h2 className="text-4xl font-bold mb-8">Architecture</h2>
        <div className="grid md:grid-cols-3 gap-6 text-left">
          <div data-kami="component:asterisk" className="bg-rh-gray-60/30 rounded-2xl p-6">
            <div className="text-rh-red-50 font-bold text-lg mb-2">Asterisk</div>
            <p className="text-rh-gray-20 text-sm">
              Domain tool. RCA pipeline, RP adapter, evidence correlation.
              The "police station" that investigates CI crimes.
            </p>
          </div>
          <div data-kami="component:origami" className="bg-rh-gray-60/30 rounded-2xl p-6">
            <div className="text-rh-purple-50 font-bold text-lg mb-2">Origami</div>
            <p className="text-rh-gray-20 text-sm">
              Framework. Graph, Walker, DSL, Personas, Elements, Masks,
              Adversarial Dialectic. The engine under the hood.
            </p>
          </div>
          <div data-kami="component:kami" className="bg-rh-gray-60/30 rounded-2xl p-6">
            <div className="text-rh-teal-50 font-bold text-lg mb-2">Kami</div>
            <p className="text-rh-gray-20 text-sm">
              Debugger. EventBridge, SSE/WS, MCP tools, React frontend.
              The divine spirits inhabiting the pipeline nodes.
            </p>
          </div>
        </div>
        <div className="mt-8 flex items-center justify-center gap-4 text-rh-gray-40">
          <span className="bg-rh-red-50/20 text-rh-red-50 px-3 py-1 rounded-full text-sm">Asterisk</span>
          <span>imports</span>
          <span className="bg-rh-purple-50/20 text-rh-purple-50 px-3 py-1 rounded-full text-sm">Origami</span>
          <span>contains</span>
          <span className="bg-rh-teal-50/20 text-rh-teal-50 px-3 py-1 rounded-full text-sm">Kami</span>
        </div>
      </div>
    </section>
  )
}
