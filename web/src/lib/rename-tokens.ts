/** Token chips offered by the rename pattern editors (docs/architecture/contract.md §7.6 rename pattern tokens). */
export const RENAME_TOKENS = [
  '{albumartist}',
  '{artist}',
  '{album}',
  '{title}',
  '{track}',
  '{track:2}',
  '{disc}',
  '{year}',
  '{genre}',
  '{composer}',
  '/',
  '[{disc}-]',
] as const
