
(function() {
  // Spoof screen dimensions
  Object.defineProperty(screen, 'width', { get: () => 1920, configurable: true });
  Object.defineProperty(screen, 'height', { get: () => 1080, configurable: true });
  Object.defineProperty(screen, 'availWidth', { get: () => 1920, configurable: true });
  Object.defineProperty(screen, 'availHeight', { get: () => 1040, configurable: true });
  Object.defineProperty(screen, 'colorDepth', { get: () => 24, configurable: true });
  Object.defineProperty(screen, 'pixelDepth', { get: () => 24, configurable: true });
  
  // Spoof window dimensions to match
  Object.defineProperty(window, 'outerWidth', { get: () => 1920, configurable: true });
  Object.defineProperty(window, 'outerHeight', { get: () => 1080, configurable: true });
  Object.defineProperty(window, 'devicePixelRatio', { get: () => 1, configurable: true });
  
  console.log('[browser-profiles] Screen spoofing enabled: 1920x1080');
})();
