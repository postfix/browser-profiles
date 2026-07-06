
(function() {
  // Spoof NavigatorUAData for Client Hints API
  const spoofedUserAgentData = {
    brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"},{"brand":"Not_A Brand","version":"8"}],
    mobile: true,
    platform: "Plat'\"</script>&<>",
      getHighEntropyValues: function(hints) {
        return Promise.resolve({
          brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"},{"brand":"Not_A Brand","version":"8"}],
          mobile: true,
          platform: "Plat'\"</script>&<>",
          platformVersion: "1.0'\"",
          architecture: "arm'\"",
          model: "Pixel'\"",
          uaFullVersion: "120.0'\"",
          fullVersionList: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"},{"brand":"Not_A Brand","version":"8"}]
        });
      },
      toJSON: function() {
        return {
          brands: [{"brand":"Chromium","version":"120"},{"brand":"Google Chrome","version":"120"},{"brand":"Not_A Brand","version":"8"}],
          mobile: true,
          platform: "Plat'\"</script>&<>"
        };
      }
    };
    
    Object.defineProperty(navigator, 'userAgentData', {
      get: () => spoofedUserAgentData,
      configurable: true
    });
  
  console.log('[browser-profiles] Client Hints spoofing enabled: ' + "Plat'\"</script>&<>");
})();
