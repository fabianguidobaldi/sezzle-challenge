// Small credit line under the keypad.

/** Claude's spark: rays around a centre, alternating long and short. Inline, so no asset to load. */
function ClaudeSpark() {
  return (
    <svg className="footer__spark" viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" focusable="false">
      {Array.from({ length: 12 }, (_, i) => (
        <line
          key={i}
          x1="12"
          y1="9.5"
          x2="12"
          y2={i % 2 === 0 ? 1.5 : 3.5}
          transform={`rotate(${i * 30} 12 12)`}
          stroke="currentColor"
          strokeWidth="2.2"
          strokeLinecap="round"
        />
      ))}
    </svg>
  )
}

export function Footer() {
  return (
    <footer className="footer">
      Made with <span role="img" aria-label="love">❤️</span> and Claude <ClaudeSpark />
    </footer>
  )
}
