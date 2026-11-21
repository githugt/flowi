// The words the window uses that a newcomer may not know, each in a sentence. One place, so a term reads the same
// wherever it is explained.
// Before: "digest", "seed", "Arazzo" and "drift" appeared with no explanation.
export const glossary = {
  arazzo: 'Arazzo is the OpenAPI Initiative\'s file format for a sequence of API calls. Flowi flows are saved as Arazzo, so other tools can read them.',
  drift: 'Drift is when an API\'s real responses stop matching the schema Flowi stored for them: a field appears, disappears or changes type.',
  seed: 'The seed is the number behind a run\'s random values ($uuid, $email…). Running again with the same seed gives the same values.',
  digest: 'A digest is a short fingerprint of a body (SHA-256). Two bodies with the same digest are the same, byte for byte.',
} as const
