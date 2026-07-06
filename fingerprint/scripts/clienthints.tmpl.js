
(function() {
  // Spoof NavigatorUAData for Client Hints API
  if (navigator.userAgentData) {
    const spoofedUserAgentData = {
      brands: %%BRANDSJSON%%,
      mobile: %%MOBILE%%,
      platform: '%%PLATFORM%%',
      getHighEntropyValues: function(hints) {
        return Promise.resolve({
          brands: %%BRANDSJSON%%,
          mobile: %%MOBILE%%,
          platform: '%%PLATFORM%%',
          platformVersion: '%%PVER%%',
          architecture: '%%ARCH%%',
          model: '%%MODEL%%',
          uaFullVersion: '120.0.6099.71',
          fullVersionList: %%BRANDSJSON%%
        });
      },
      toJSON: function() {
        return {
          brands: %%BRANDSJSON%%,
          mobile: %%MOBILE%%,
          platform: '%%PLATFORM%%'
        };
      }
    };
    
    Object.defineProperty(navigator, 'userAgentData', {
      get: () => spoofedUserAgentData,
      configurable: true
    });
  }
  
  console.log('[browser-profiles] Client Hints spoofing enabled: %%PLATFORM%%');
})();
