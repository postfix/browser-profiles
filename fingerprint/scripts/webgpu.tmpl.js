(function() {
  const cfg = %%WGPUJSON%%;

  function makeAdapterInfo() {
    return {
      vendor: cfg.vendor || '',
      architecture: cfg.architecture || '',
      device: cfg.device || '',
      description: cfg.description || '',
      // Spec helpers: subgroupMinSize/subgroupMaxSize are not present on all adapters,
      // omit them to keep the mock minimal and coherent across browsers.
    };
  }

  const info = makeAdapterInfo();
  const mockAdapter = {
    info: info,
    requestAdapterInfo: function() {
      return Promise.resolve(info);
    }
  };

  // Browsers without WebGPU expose no navigator.gpu at all. Provide a minimal
  // GPU-shaped object so the spoof is consistent in those environments too.
  const gpuBase = (typeof navigator !== 'undefined' && navigator.gpu) || {};

  const spoofedGPU = Object.create(Object.getPrototypeOf(gpuBase) || Object.prototype);
  Object.keys(gpuBase).forEach(function(k) {
    const desc = Object.getOwnPropertyDescriptor(gpuBase, k);
    if (desc) {
      Object.defineProperty(spoofedGPU, k, desc);
    }
  });

  spoofedGPU.requestAdapter = function() {
    return Promise.resolve(mockAdapter);
  };

  try {
    Object.defineProperty(navigator, 'gpu', {
      value: spoofedGPU,
      configurable: true,
      enumerable: true,
      writable: true
    });
  } catch (e) {
    // Some environments protect the navigator.gpu property; best-effort only.
  }

  console.log('[browser-profiles] WebGPU protection enabled');
})();
