const FRAMEWORKS = [
  {
    name: 'Origami',
    model: 'Walker-centric graph',
    dsl: 'YAML DSL',
    identity: 'Personas + Elements',
    debug: 'Kami live debugger',
    highlight: true,
  },
  {
    name: 'LangGraph',
    model: 'State-centric graph',
    dsl: 'Python builder',
    identity: 'None',
    debug: 'LangSmith (paid)',
    highlight: false,
  },
  {
    name: 'CrewAI',
    model: 'Role-based crews',
    dsl: 'YAML + Python',
    identity: 'Role strings',
    debug: 'Enterprise (paid)',
    highlight: false,
  },
]

export function Competitive() {
  return (
    <section id="competitive" data-kami="section:competitive" className="section flex items-center justify-center bg-white px-8" aria-label="Competitive Landscape">
      <div className="max-w-5xl w-full">
        <h2 className="text-4xl font-bold text-rh-gray-80 mb-8 text-center">Competitive Landscape</h2>
        <div className="overflow-x-auto">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b-2 border-rh-gray-20">
                <th className="py-3 px-4 text-rh-gray-60 font-semibold">Framework</th>
                <th className="py-3 px-4 text-rh-gray-60 font-semibold">Model</th>
                <th className="py-3 px-4 text-rh-gray-60 font-semibold">DSL</th>
                <th className="py-3 px-4 text-rh-gray-60 font-semibold">Agent Identity</th>
                <th className="py-3 px-4 text-rh-gray-60 font-semibold">Debugging</th>
              </tr>
            </thead>
            <tbody>
              {FRAMEWORKS.map((fw) => (
                <tr
                  key={fw.name}
                  data-kami={`framework:${fw.name}`}
                  className={`border-b border-rh-gray-20 ${fw.highlight ? 'bg-rh-red-10 font-semibold' : ''}`}
                >
                  <td className="py-3 px-4">{fw.highlight ? <span className="text-rh-red-50">{fw.name}</span> : fw.name}</td>
                  <td className="py-3 px-4">{fw.model}</td>
                  <td className="py-3 px-4">{fw.dsl}</td>
                  <td className="py-3 px-4">{fw.identity}</td>
                  <td className="py-3 px-4">{fw.debug}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  )
}
