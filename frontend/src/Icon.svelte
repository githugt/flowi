<script lang="ts">
  // Small stroke icons (24x24 grid), drawn here so the window needs no icon font.
  let { name, size = 16 }: { name: string; size?: number } = $props()
  const paths: Record<string, string> = {
    import: 'M12 3v12 M7 10l5 5 5-5 M4 20h16',
    folder: 'M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z',
    key: 'M15 7a4 4 0 1 1-3.5 6L4 20.5V17h3v-3h3l1.5-1.5A4 4 0 0 1 15 7z M16 9h.01',
    clock: 'M12 7v5l3 2 M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0z',
    replay: 'M5 12a7 7 0 0 1 12-5l3 3 M20 4v6h-6 M19 12a7 7 0 0 1-12 5l-3-3 M4 20v-6h6',
    blueprint: 'M4 4h7v7H4z M13 4h7v4h-7z M13 10h7v10h-7z M4 13h7v7H4z',
    settings: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3h0a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5h0a1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8v0a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z',
    plus: 'M12 5v14 M5 12h14',
    trash: 'M4 7h16 M9 7V4h6v3 M6 7l1 13h10l1-13 M10 11v6 M14 11v6',
    params: 'M9.5 4L7.5 20M16.5 4l-2 16M4 9h16M3.5 15h16',
    headers: 'M7 3h7l4 4v14H7z M14 3v4h4 M9.5 12h5 M9.5 16h5',
    body: 'M9 4C7 4 7 6 7 8s0 3-3 4c3 1 3 2 3 4s0 4 2 4 M15 4c2 0 2 2 2 4s0 3 3 4c-3 1-3 2-3 4s0 4-2 4',
    auth: 'M12 3l7 3v5c0 5-3 8-7 10-4-2-7-5-7-10V6z',
    hooks: 'M10 14a4 4 0 0 0 6 0l3-3a4 4 0 0 0-6-6l-1 1 M14 10a4 4 0 0 0-6 0l-3 3a4 4 0 0 0 6 6l1-1',
    docs: 'M5 4h10a3 3 0 0 1 3 3v13H8a3 3 0 0 1-3-3z M8 4v16 M11 9h4',
    schema: 'M4 6h6 M7 12h8 M7 18h8 M4 6v12 M4 12h3 M4 18h3',
    stream: 'M12 3v10 M8 10l4 4 4-4 M8 16l4 4 4-4',
    proto: 'M7 3h7l4 4v14H7z M14 3v4h4 M11 12l-2 2 2 2 M14 12l2 2-2 2',
    binding: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1 M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1',
    code: 'M8 7l-5 5 5 5 M16 7l5 5-5 5',
    cookie: 'M20 12a8 8 0 1 1-8-8 3 3 0 0 0 4 4 3 3 0 0 0 4 4z M9 10h.01 M13 14h.01 M9 15h.01',
    usedby: 'M4 12h6 M14 6h6 M14 18h6 M10 12l4-6 M10 12l4 6',
    list: 'M8 6h12 M8 12h12 M8 18h12 M4 6h.01 M4 12h.01 M4 18h.01',
    send: 'M22 2L11 13 M22 2l-7 20-4-9-9-4z',
    // The kinds of gRPC call: an arrow up for what is sent, down for what comes back, doubled when it streams.
    'rpc-unary': 'M8 20V5 M5 8l3-3 3 3 M16 4v15 M13 16l3 3 3-3',
    'rpc-server': 'M8 20V5 M5 8l3-3 3 3 M16 4v10 M13 11l3 3 3-3 M13 16l3 3 3-3',
    'rpc-client': 'M8 20V5 M5 8l3-3 3 3 M5 13l3-3 3 3 M16 4v15 M13 16l3 3 3-3',
    'rpc-bidi': 'M8 20V5 M5 8l3-3 3 3 M5 13l3-3 3 3 M16 4v10 M13 11l3 3 3-3 M13 16l3 3 3-3',
    refresh: 'M20 12a8 8 0 1 1-2.3-5.7 M20 4v5h-5',
    save: 'M5 3h11l4 4v13a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1z M8 3v5h7V3 M7 21v-7h10v7',
    home: 'M3 11l9-8 9 8 M5 9.5V20h5v-6h4v6h5V9.5',
    chart: 'M4 20V10 M10 20V4 M16 20v-7 M21 20H3',
    extract: 'M4 4h10v6 M4 4v16h10 M11 15h10 M17 11l4 4-4 4',
    assert: 'M9 11l3 3 8-8 M20 12v7a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h9',
    variables: 'M8 4c-2 0-3 1-3 3v3l-2 2 2 2v3c0 2 1 3 3 3 M16 4c2 0 3 1 3 3v3l2 2-2 2v3c0 2-1 3-3 3 M10 9l4 6 M14 9l-4 6',
    notsent: 'M21 4L3 11l7 2.5L12.5 21z M10 13.5l4.5-4.5 M3 17h3 M3 21h6',
  }
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  <path d={paths[name]} />
</svg>
