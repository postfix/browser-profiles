
(function() {
  // Spoof NavigatorUAData for Client Hints API
  if (navigator.userAgentData) {
    const spoofedUserAgentData = {
      brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"},{"brand":"Not_A Brand","version":"8"}],
      mobile: false,
      platform: 'Windows',
      getHighEntropyValues: function(hints) {
        return Promise.resolve({
          brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"},{"brand":"Not_A Brand","version":"8"}],
          mobile: false,
          platform: 'Windows',
          platformVersion: '10.0.0',
          architecture: 'x86',
          model: '',
          uaFullVersion: '120.0.6099.71',
          fullVersionList: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"},{"brand":"Not_A Brand","version":"8"}]
        });
      },
      toJSON: function() {
        return {
          brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"},{"brand":"Not_A Brand","version":"8"}],
          mobile: false,
          platform: 'Windows'
        };
      }
    };
    
    Object.defineProperty(navigator, 'userAgentData', {
      get: () => spoofedUserAgentData,
      configurable: true
    });
  }
  
  console.log('[browser-profiles] Client Hints spoofing enabled: Windows');
})();
