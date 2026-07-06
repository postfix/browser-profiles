(function() {
  const cfg = {"enabled":true,"precisionMs":1};

  if (!cfg.enabled) {
    console.log('[browser-profiles] Timing spoofing disabled');
    return;
  }

  const precision = cfg.precisionMs || 1.0;

  const realPerformanceNow = performance.now.bind(performance);
  const realDateNow = Date.now.bind(Date);
  const RealDate = Date;

  const perfBase = realPerformanceNow();
  const dateBase = realDateNow();
  const roundPerfBase = Math.round(perfBase / precision) * precision;
  const roundDateBase = Math.round(dateBase / precision) * precision;

  function roundToPrecision(value) {
    return Math.round(value / precision) * precision;
  }

  performance.now = function() {
    return roundPerfBase + roundToPrecision(realPerformanceNow() - perfBase);
  };

  function spoofedDateNow() {
    return roundDateBase + roundToPrecision(realDateNow() - dateBase);
  }

  Date.now = spoofedDateNow;

  function SpoofedDate() {
    if (arguments.length === 0) {
      return new RealDate(spoofedDateNow());
    }
    return new RealDate(...arguments);
  }

  SpoofedDate.prototype = RealDate.prototype;
  SpoofedDate.now = spoofedDateNow;
  SpoofedDate.parse = RealDate.parse;
  SpoofedDate.UTC = RealDate.UTC;

  // Replace the global Date constructor without breaking prototype chains.
  try {
    const g = (function() { return this; })() || self || window || global;
    g.Date = SpoofedDate;
  } catch (e) {
    // Best-effort fallback should not throw.
  }

  console.log('[browser-profiles] Timing spoofing enabled (precision ' + precision + 'ms)');
})();
