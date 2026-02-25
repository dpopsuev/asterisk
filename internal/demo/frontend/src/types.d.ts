import type { KamiSelection } from './hooks/useKamiSelector'

declare global {
  interface Window {
    __origami?: {
      selection?: KamiSelection
      [key: string]: unknown
    }
  }
}
