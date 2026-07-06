
(function() {
  // Spoof screen dimensions
  Object.defineProperty(screen, 'width', { get: () => %%WIDTH%%, configurable: true });
  Object.defineProperty(screen, 'height', { get: () => %%HEIGHT%%, configurable: true });
  Object.defineProperty(screen, 'availWidth', { get: () => %%AVAILW%%, configurable: true });
  Object.defineProperty(screen, 'availHeight', { get: () => %%AVAILH%%, configurable: true });
  Object.defineProperty(screen, 'colorDepth', { get: () => %%COLORDEPTH%%, configurable: true });
  Object.defineProperty(screen, 'pixelDepth', { get: () => %%PIXELDEPTH%%, configurable: true });
  
  // Spoof window dimensions to match
  Object.defineProperty(window, 'outerWidth', { get: () => %%WIDTH%%, configurable: true });
  Object.defineProperty(window, 'outerHeight', { get: () => %%HEIGHT%%, configurable: true });
  Object.defineProperty(window, 'devicePixelRatio', { get: () => %%DPR%%, configurable: true });
  
  console.log('[browser-profiles] Screen spoofing enabled: %%WIDTH%%x%%HEIGHT%%');
})();
