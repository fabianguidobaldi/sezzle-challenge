// The calculator screen: pending expression (or error) above the current value.

export interface DisplayProps {
  value: string
  exact: boolean
  expression: string // the pending expression, or the error message when error is true
  busy: boolean
  error?: boolean
}

/** Font-size step for a value, so the longest value (19 chars) fits a 320px screen. */
function sizeClass(value: string): string {
  if (value.length <= 7) return 'display__value--xl'
  if (value.length <= 10) return 'display__value--lg'
  if (value.length <= 14) return 'display__value--md'
  return 'display__value--sm'
}

export function Display({ value, exact, expression, busy, error = false }: DisplayProps) {
  return (
    <section className="display" aria-label="Display" aria-busy={busy}>
      <span className={busy ? 'display__pending display__pending--on' : 'display__pending'} aria-hidden="true" />
      <div
        className={error ? 'display__expression display__expression--error' : 'display__expression'}
        role={error ? 'alert' : undefined}
        data-testid="display-expression"
      >
        {expression}
      </div>
      <output className={`display__value ${sizeClass(value)}`} aria-live="polite">
        {!exact && (
          <span className="display__approx">
            <span aria-hidden="true">≈</span>
            <span className="visually-hidden">approximately </span>
          </span>
        )}
        {value}
      </output>
    </section>
  )
}
