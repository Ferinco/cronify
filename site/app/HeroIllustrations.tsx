function GearIcon({ size = 64, teeth = 8 }: { size?: number; teeth?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      {Array.from({ length: teeth }).map((_, i) => (
        <rect
          key={i}
          x="29"
          y="6"
          width="6"
          height="12"
          rx="1.5"
          fill="currentColor"
          transform={`rotate(${(360 / teeth) * i} 32 32)`}
        />
      ))}
      <circle cx="32" cy="32" r="17" stroke="currentColor" strokeWidth="2" />
      <circle cx="32" cy="32" r="6" stroke="currentColor" strokeWidth="2" />
    </svg>
  );
}

function BotIcon({ size = 64 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="none">
      <line x1="32" y1="6" x2="32" y2="15" stroke="currentColor" strokeWidth="2" />
      <circle cx="32" cy="4" r="2.5" fill="currentColor" />
      <rect x="18" y="15" width="28" height="20" rx="6" stroke="currentColor" strokeWidth="2" />
      <circle className="illo-eye" cx="26.5" cy="25" r="2.5" />
      <circle cx="37.5" cy="25" r="2.5" fill="currentColor" />
      <rect x="14" y="37" width="36" height="22" rx="6" stroke="currentColor" strokeWidth="2" />
      <line x1="22" y1="46" x2="42" y2="46" stroke="currentColor" strokeWidth="2" />
      <line x1="22" y1="52" x2="34" y2="52" stroke="currentColor" strokeWidth="2" />
      <line x1="14" y1="43" x2="6" y2="43" stroke="currentColor" strokeWidth="2" />
      <circle cx="5" cy="43" r="2" fill="currentColor" />
      <line x1="50" y1="43" x2="58" y2="43" stroke="currentColor" strokeWidth="2" />
      <circle cx="59" cy="43" r="2" fill="currentColor" />
    </svg>
  );
}

/**
 * Decorative background illustrations for the hero: gears and a small bot,
 * evoking the "moving parts of a machine" behind scheduled/cron jobs. Purely
 * decorative — aria-hidden, no interaction, hidden on narrow viewports where
 * there's no side margin for them to live in without crowding the headline.
 */
export function HeroIllustrations() {
  return (
    <div className="hero-illustrations" aria-hidden="true">
      <div className="illo illo-gear-1">
        <GearIcon size={92} teeth={10} />
      </div>
      <div className="illo illo-gear-2">
        <GearIcon size={56} teeth={8} />
      </div>
      <div className="illo illo-gear-3">
        <GearIcon size={40} teeth={6} />
      </div>
      <div className="illo illo-bot-1">
        <BotIcon size={78} />
      </div>
      <div className="illo illo-bot-2">
        <BotIcon size={50} />
      </div>
    </div>
  );
}
