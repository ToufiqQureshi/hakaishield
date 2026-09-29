interface EvidenceSignalsProps {
  signals?: string[];
  shadowSignals?: string[];
}

const shown = 3;

// A row shows the first few of each kind; the rest are counted, never
// dropped silently, and named in the badge tooltip.
export default function EvidenceSignals({ signals = [], shadowSignals = [] }: EvidenceSignalsProps) {
  const hiddenScored = signals.slice(shown);
  const hiddenShadow = shadowSignals.slice(shown);
  return (
    <div className="flex flex-wrap gap-1">
      {signals.slice(0, shown).map((signal) => (
        <span key={`scored-${signal}`} className="badge badge-red text-[10px]">{signal}</span>
      ))}
      {hiddenScored.length > 0 && (
        <span className="badge badge-red text-[10px]" title={hiddenScored.join(', ')}>+{hiddenScored.length} more</span>
      )}
      {shadowSignals.slice(0, shown).map((signal) => (
        <span key={`observed-${signal}`} className="badge badge-yellow text-[10px]" title="Observed only; did not affect the decision">
          Observed: {signal}
        </span>
      ))}
      {hiddenShadow.length > 0 && (
        <span className="badge badge-yellow text-[10px]" title={`Observed only: ${hiddenShadow.join(', ')}`}>
          +{hiddenShadow.length} more observed
        </span>
      )}
    </div>
  );
}
