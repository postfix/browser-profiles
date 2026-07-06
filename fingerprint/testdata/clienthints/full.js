
(function() {
  // Spoof NavigatorUAData for Client Hints API
  const spoofedUserAgentData = {
    brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"}],
    mobile: false,
    platform: "macOS",
      getHighEntropyValues: function(hints) {
        return Promise.resolve({
          brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"}],
          mobile: false,
          platform: "macOS",
          platformVersion: "14.2.0",
          architecture: "arm",
          model: "",
          uaFullVersion: "120.0.6099.71",
          fullVersionList: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"}]
        });
      },
      toJSON: function() {
        return {
          brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"}],
          mobile: false,
          platform: "macOS"
        };
      }
    };
    
    Object.defineProperty(navigator, 'userAgentData', {
      get: () => spoofedUserAgentData,
      configurable: true
    });
  
  console.log('[browser-profiles] Client Hints spoofing enabled: ' + "macOS");
})();
