import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'
import { reportUnhandled } from './lib/errors'
import { ui } from './lib/ui.svelte'
import { dropPointerFocusOnBlur } from './lib/focusring'

// Calls to Go that nobody awaited still show their errors.
window.addEventListener('unhandledrejection', (e) => reportUnhandled(e.reason, (text, level) => ui.toast(text, level)))

// A clicked control does not come back from Alt+Tab wearing a focus ring.
dropPointerFocusOnBlur()

mount(App, { target: document.getElementById('app')! })
