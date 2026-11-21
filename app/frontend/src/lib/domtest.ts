import { createRawSnippet } from 'svelte'

// For the DOM tests only: a snippet that renders a fixed bit of HTML, to hand to a component that takes `children`
// or `actions`.
export const text = (html: string) => createRawSnippet(() => ({ render: () => `<span>${html}</span>` }))
