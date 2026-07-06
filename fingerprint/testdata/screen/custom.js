
(function() {
  // Spoof screen dimensions
  Object.defineProperty(screen, 'width', { get: () => 2560, configurable: true });
  Object.defineProperty(screen, 'height', { get: () => 1440, configurable: true });
  Object.defineProperty(screen, 'availWidth', { get: () => 2560, configurable: true });
  Object.defineProperty(screen, 'availHeight', { get: () => 1400, configurable: true });
  Object.defineProperty(screen, 'colorDepth', { get: () => 24, configurable: true });
  Object.defineProperty(screen, 'pixelDepth', { get: () => 24, configurable: true });
  
  // Spoof window dimensions to match
  Object.defineProperty(window, 'outerWidth', { get: () => 2560, configurable: true });
  Object.defineProperty(window, 'outerHeight', { get: () => 1440, configurable: true });
  Object.defineProperty(window, 'devicePixelRatio', { get: () => 1, configurable: true });
  
  console.log('[browser-profiles] Screen spoofing enabled: 2560x1440');
})();
