
(function() {
  const spoofedProps = {"userAgent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36","language":"en-US","platform":"Win32","hardwareConcurrency":8,"deviceMemory":8,"vendor":"Google Inc.","appVersion":"(Windows NT 10.0; Win64; x64) AppleWebKit/537.36","productSub":"20030107","connection":{"effectiveType":"4g","downlink":10,"rtt":50}};
  const navigatorProto = Object.getPrototypeOf(navigator);
  
  // Helper to replace getter with a new value
  function replaceGetter(propName, newValue) {
    try {
      Object.defineProperty(navigatorProto, propName, {
        get: () => newValue,
        configurable: true
      });
    } catch(e) {
      // Fallback: try on navigator directly
      try {
        Object.defineProperty(navigator, propName, {
          value: newValue,
          configurable: true,
          writable: false
        });
      } catch(e2) {}
    }
  }
  
  if (spoofedProps.userAgent) {
    replaceGetter('userAgent', spoofedProps.userAgent);
  }
  
  if (spoofedProps.language) {
    replaceGetter('language', spoofedProps.language);
    replaceGetter('languages', [spoofedProps.language, spoofedProps.language.split('-')[0]]);
  }
  
  if (spoofedProps.platform) {
    replaceGetter('platform', spoofedProps.platform);
  }
  
  if (spoofedProps.hardwareConcurrency) {
    replaceGetter('hardwareConcurrency', spoofedProps.hardwareConcurrency);
  }
  
  if (spoofedProps.deviceMemory) {
    replaceGetter('deviceMemory', spoofedProps.deviceMemory);
  }
  
  if (spoofedProps.vendor) {
    replaceGetter('vendor', spoofedProps.vendor);
  }
  
  if (spoofedProps.appVersion) {
    replaceGetter('appVersion', spoofedProps.appVersion);
  }
  
  if (spoofedProps.productSub) {
    replaceGetter('productSub', spoofedProps.productSub);
  }
  
  if (spoofedProps.maxTouchPoints !== undefined && spoofedProps.maxTouchPoints !== null) {
    replaceGetter('maxTouchPoints', spoofedProps.maxTouchPoints);
  }
  
  if (spoofedProps.mobile !== undefined && spoofedProps.mobile !== null) {
    replaceGetter('mobile', spoofedProps.mobile);
  }
  
  if (spoofedProps.connection) {
    const connectionObj = Object.freeze(Object.assign({}, spoofedProps.connection));
    replaceGetter('connection', connectionObj);
  }
  
  console.log('[browser-profiles] Navigator spoofing enabled:', spoofedProps);
})();
