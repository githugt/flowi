<script lang="ts">
  // A verb (GET, MQTT …) in its colour. "pill" has the tinted, glowing outline of the tree and the pickers;
  // "text" is the bare coloured text (the tabs).
  let { verb, text: shown, pill = true, short = false }: { verb: string; text?: string; pill?: boolean; short?: boolean } = $props()
  const known = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'WS', 'SSE', 'MQTT', 'KAFKA', 'AMQP', 'NATS', 'GRPC', 'REDIS', 'SQS', 'SNS', 'BUS', 'PUBSUB', 'FLOW']
  const key = $derived(known.includes(verb.toUpperCase()) ? verb.toUpperCase() : 'OTHER')
  // httpie-style short forms keep the tab narrow.
  const text = $derived(shown ?? (short ? (verb === 'DELETE' ? 'DEL' : verb.slice(0, 5)) : verb))
</script>

<span class="verb" class:pill style:--v={`var(--v-${key})`}>{text}</span>

<style>
  .verb { color: var(--v); font-weight: 700; font-size: var(--fs-xs); letter-spacing: 0.02em; white-space: nowrap; }
  .pill {
    display: inline-block;
    min-width: 3.2em;
    box-sizing: border-box;
    text-align: center;
    padding: 1px 6px;
    border-radius: var(--r-md);
    background: color-mix(in srgb, var(--v) 15%, transparent);
    border: 1px solid color-mix(in srgb, var(--v) 50%, transparent);
    box-shadow: 0 0 6px color-mix(in srgb, var(--v) 30%, transparent);
  }
</style>
