// A React host of the cart widget of the shop (REQ-ISL-16). The widget is a
// custom element: React 19 writes its attributes and listens to its events
// from the props of the JSX tag.
import { useState } from 'react'
import { createRoot } from 'react-dom/client'

type Detail<T> = { detail: T }

function App() {
  const [total, setTotal] = useState(0)
  const [state, setState] = useState('loading')
  const [label, setLabel] = useState('React cart')
  const tag = {
    label,
    'oncart-changed': (e: Detail<{ total: number }>) => setTotal(e.detail.total),
    'ongx-ready': () => setState('ready'),
    'ongx-error': (e: Detail<{ status: number; key: string }>) => setState(`error ${e.detail.status} ${e.detail.key}`),
  }
  return (
    <main>
      <h1>React host</h1>
      <p id="total">{total}</p>
      <p id="state">{state}</p>
      <button id="relabel" onClick={() => setLabel('Second label')}>
        Relabel
      </button>
      {/* A host project gets the JSX types of the tag from shop-cart.react.d.ts. */}
      <shop-cart {...tag}>
        <p id="fallback">Loading the cart</p>
      </shop-cart>
    </main>
  )
}

createRoot(document.getElementById('root')!).render(<App />)
