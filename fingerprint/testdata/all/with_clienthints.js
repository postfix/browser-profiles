
(function() {
  // Disable WebRTC IP leak by overriding RTCPeerConnection
  const originalRTCPeerConnection = window.RTCPeerConnection;
  
  if (!originalRTCPeerConnection) {
    console.log('[browser-profiles] WebRTC not available, skipping protection');
    return;
  }
  
  // Create a wrapper that filters out local IP candidates
  window.RTCPeerConnection = function(configuration, constraints) {
    // Force use of TURN only if possible to hide real IP
    if (configuration && configuration.iceServers) {
      configuration.iceCandidatePoolSize = 0;
    }
    
    const pc = new originalRTCPeerConnection(configuration, constraints);
    
    // Override onicecandidate to filter local IPs
    const originalAddEventListener = pc.addEventListener.bind(pc);
    pc.addEventListener = function(type, listener, options) {
      if (type === 'icecandidate') {
        const wrappedListener = function(event) {
          if (event.candidate && event.candidate.candidate) {
            // Filter out candidates with real IPs (keep relay candidates)
            const candidate = event.candidate.candidate;
            if (candidate.includes('typ host') || candidate.includes('typ srflx')) {
              // Skip host and server reflexive candidates that reveal real IP
              console.log('[browser-profiles] Blocked WebRTC IP leak candidate');
              return;
            }
          }
          listener.call(this, event);
        };
        return originalAddEventListener(type, wrappedListener, options);
      }
      return originalAddEventListener(type, listener, options);
    };
    
    // Also handle the onicecandidate property
    let _onicecandidate = null;
    Object.defineProperty(pc, 'onicecandidate', {
      get: function() { return _onicecandidate; },
      set: function(handler) {
        _onicecandidate = function(event) {
          if (event.candidate && event.candidate.candidate) {
            const candidate = event.candidate.candidate;
            if (candidate.includes('typ host') || candidate.includes('typ srflx')) {
              console.log('[browser-profiles] Blocked WebRTC IP leak candidate');
              return;
            }
          }
          if (handler) handler.call(this, event);
        };
      }
    });
    
    return pc;
  };
  
  // Copy static properties and prototype
  window.RTCPeerConnection.prototype = originalRTCPeerConnection.prototype;
  Object.keys(originalRTCPeerConnection).forEach(key => {
    try {
      window.RTCPeerConnection[key] = originalRTCPeerConnection[key];
    } catch(e) {}
  });
  
  console.log('[browser-profiles] WebRTC protection enabled');
})();



(function() {
  // Store original methods
  const originalGetImageData = CanvasRenderingContext2D.prototype.getImageData;
  const originalToBlob = HTMLCanvasElement.prototype.toBlob;
  const originalToDataURL = HTMLCanvasElement.prototype.toDataURL;
  
  // Generate random shift values (consistent per page load)
  const shift = {
    r: Math.floor(Math.random() * 10) - 5,
    g: Math.floor(Math.random() * 10) - 5,
    b: Math.floor(Math.random() * 10) - 5,
    a: Math.floor(Math.random() * 10) - 5
  };
  
  // Noisify canvas data
  function noisify(canvas, context) {
    if (!context || !canvas) return;
    
    const width = canvas.width;
    const height = canvas.height;
    
    if (width && height && width * height < 1000000) { // Limit to reasonable size
      try {
        const imageData = originalGetImageData.call(context, 0, 0, width, height);
        
        for (let i = 0; i < imageData.data.length; i += 4) {
          imageData.data[i + 0] = Math.max(0, Math.min(255, imageData.data[i + 0] + shift.r));
          imageData.data[i + 1] = Math.max(0, Math.min(255, imageData.data[i + 1] + shift.g));
          imageData.data[i + 2] = Math.max(0, Math.min(255, imageData.data[i + 2] + shift.b));
          imageData.data[i + 3] = Math.max(0, Math.min(255, imageData.data[i + 3] + shift.a));
        }
        
        context.putImageData(imageData, 0, 0);
      } catch (e) {
        // Ignore cross-origin errors
      }
    }
  }
  
  // Override toBlob
  HTMLCanvasElement.prototype.toBlob = function(...args) {
    noisify(this, this.getContext('2d'));
    return originalToBlob.apply(this, args);
  };
  
  // Override toDataURL
  HTMLCanvasElement.prototype.toDataURL = function(...args) {
    noisify(this, this.getContext('2d'));
    return originalToDataURL.apply(this, args);
  };
  
  // Override getImageData
  CanvasRenderingContext2D.prototype.getImageData = function(...args) {
    noisify(this.canvas, this);
    return originalGetImageData.apply(this, args);
  };
  
  console.log('[browser-profiles] Canvas protection enabled');
})();



(function() {
  // Random helper
  function randomItem(arr) {
    return arr[Math.floor(Math.random() * arr.length)];
  }
  
  function randomPower(powers) {
    return Math.pow(2, randomItem(powers));
  }
  
  function randomInt32(powers) {
    const n = randomPower(powers);
    return new Int32Array([n, n]);
  }
  
  function randomFloat32(powers) {
    const n = randomPower(powers);
    return new Float32Array([1, n]);
  }
  
  // Spoof getParameter
  function spoofGetParameter(proto) {
    const originalGetParameter = proto.getParameter;
    
    proto.getParameter = function(pname) {
      // Spoof vendor/renderer strings
      if (pname === 37445) return "Google Inc."; // UNMASKED_VENDOR_WEBGL
      if (pname === 37446) return randomItem(["ANGLE (Intel, Intel(R) HD Graphics)", "ANGLE (NVIDIA, GeForce GTX 1080)", "ANGLE (AMD, Radeon RX 580)"]); // UNMASKED_RENDERER_WEBGL
      if (pname === 7936) return "WebKit"; // VENDOR
      if (pname === 7937) return "WebKit WebGL"; // RENDERER
      if (pname === 7938) return randomItem(["WebGL 1.0", "WebGL 1.0 (OpenGL ES 2.0 Chromium)"]); // VERSION
      if (pname === 35724) return randomItem(["WebGL GLSL ES 1.0", "WebGL GLSL ES 1.0 (OpenGL ES GLSL ES 1.0 Chromium)"]); // SHADING_LANGUAGE_VERSION
      
      // Spoof numeric parameters with randomized values
      if (pname === 3379) return randomPower([14, 15]); // MAX_TEXTURE_SIZE
      if (pname === 34076) return randomPower([14, 15]); // MAX_CUBE_MAP_TEXTURE_SIZE
      if (pname === 34024) return randomPower([14, 15]); // MAX_RENDERBUFFER_SIZE
      if (pname === 36347) return randomPower([12, 13]); // MAX_VARYING_VECTORS
      if (pname === 36348) return 30; // MAX_VERTEX_UNIFORM_VECTORS
      if (pname === 3386) return randomInt32([13, 14, 15]); // MAX_VIEWPORT_DIMS
      if (pname === 33902) return randomFloat32([0, 10, 11, 12, 13]); // ALIASED_LINE_WIDTH_RANGE
      if (pname === 33901) return randomFloat32([0, 10, 11, 12, 13]); // ALIASED_POINT_SIZE_RANGE
      if (pname === 3413) return randomPower([1, 2, 3, 4]); // MAX_TEXTURE_IMAGE_UNITS
      if (pname === 35660) return randomPower([1, 2, 3, 4]); // MAX_VERTEX_TEXTURE_IMAGE_UNITS
      if (pname === 35661) return randomPower([4, 5, 6, 7, 8]); // MAX_COMBINED_TEXTURE_IMAGE_UNITS
      if (pname === 34930) return randomPower([1, 2, 3, 4]); // MAX_FRAGMENT_UNIFORM_VECTORS
      if (pname === 36349) return randomPower([10, 11, 12, 13]); // MAX_VERTEX_ATTRIBS
      
      return originalGetParameter.call(this, pname);
    };
  }
  
  // Add noise to buffer data
  function spoofBufferData(proto) {
    const originalBufferData = proto.bufferData;
    
    proto.bufferData = function(target, data, usage) {
      if (data && data.length) {
        const index = Math.floor(Math.random() * data.length);
        if (data[index] !== undefined) {
          data[index] = data[index] + 0.1 * Math.random() * data[index];
        }
      }
      return originalBufferData.call(this, target, data, usage);
    };
  }
  
  // Apply to WebGL contexts
  if (typeof WebGLRenderingContext !== 'undefined') {
    spoofGetParameter(WebGLRenderingContext.prototype);
    spoofBufferData(WebGLRenderingContext.prototype);
  }
  
  if (typeof WebGL2RenderingContext !== 'undefined') {
    spoofGetParameter(WebGL2RenderingContext.prototype);
    spoofBufferData(WebGL2RenderingContext.prototype);
  }
  
  console.log('[browser-profiles] WebGL protection enabled');
})();



(function() {
  let processedBuffer = null;
  
  // Override getChannelData
  if (typeof AudioBuffer !== 'undefined') {
    const originalGetChannelData = AudioBuffer.prototype.getChannelData;
    
    AudioBuffer.prototype.getChannelData = function(channel) {
      const data = originalGetChannelData.call(this, channel);
      
      if (processedBuffer !== data) {
        processedBuffer = data;
        
        // Add tiny noise that doesn't affect audio quality
        for (let i = 0; i < data.length; i += 100) {
          const index = Math.floor(Math.random() * i);
          if (data[index] !== undefined) {
            data[index] = data[index] + Math.random() * 0.0000001;
          }
        }
      }
      
      return data;
    };
  }
  
  // Override createAnalyser to add noise to frequency data
  function spoofCreateAnalyser(AudioContextClass) {
    if (typeof AudioContextClass === 'undefined') return;
    
    const originalCreateAnalyser = AudioContextClass.prototype.createAnalyser;
    
    AudioContextClass.prototype.createAnalyser = function() {
      const analyser = originalCreateAnalyser.call(this);
      
      const originalGetFloatFrequencyData = analyser.getFloatFrequencyData.bind(analyser);
      
      analyser.getFloatFrequencyData = function(array) {
        originalGetFloatFrequencyData(array);
        
        for (let i = 0; i < array.length; i += 100) {
          const index = Math.floor(Math.random() * i);
          if (array[index] !== undefined) {
            array[index] = array[index] + Math.random() * 0.1;
          }
        }
      };
      
      return analyser;
    };
  }
  
  spoofCreateAnalyser(window.AudioContext);
  spoofCreateAnalyser(window.OfflineAudioContext);
  
  console.log('[browser-profiles] Audio protection enabled');
})();


(function() {
  const cfg = {"vendor":"intel","architecture":"x86","device":"Intel(R) UHD Graphics 630","description":"Intel UHD Graphics 630"};

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



(function() {
  const spoofedProps = {"language":"en-US","platform":"Win32","hardwareConcurrency":8,"deviceMemory":8};
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



(function() {
  // Spoof NavigatorUAData for Client Hints API
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
  
  console.log('[browser-profiles] Client Hints spoofing enabled: Windows');
})();



(function() {
  // ===== CDP BINDING DETECTION EVASION =====
  // Remove CDP bindings that are injected by Puppeteer
  const cdpBindings = [
    '__puppeteer_utility_world__',
    '__puppeteer_evaluation_script__',
    '__CDP_BINDING__',
    'cdc_adoQpoasnfa76pfcZLmcfl_Array',
    'cdc_adoQpoasnfa76pfcZLmcfl_Promise',
    'cdc_adoQpoasnfa76pfcZLmcfl_Symbol',
    '__driver_evaluate',
    '__webdriver_evaluate',
    '__selenium_evaluate',
    '__fxdriver_evaluate',
    '__driver_unwrapped',
    '__webdriver_unwrapped',
    '__selenium_unwrapped',
    '__fxdriver_unwrapped',
    '_Selenium_IDE_Recorder',
    '_selenium',
    'calledSelenium',
    '$chrome_asyncScriptInfo',
    '$cdc_asdjflasutopfhvcZLmcfl_',
    '__$webdriverAsyncExecutor'
  ];
  
  cdpBindings.forEach(binding => {
    try {
      if (binding in window) {
        delete window[binding];
      }
    } catch {}
  });
  
  // Hide document automation properties
  const docProps = ['webdriver', '$cdc_asdjflasutopfhvcZLmcfl_', '$chrome_asyncScriptInfo'];
  docProps.forEach(prop => {
    try {
      if (prop in document) {
        Object.defineProperty(document, prop, { get: () => undefined });
      }
    } catch {}
  });
  
  // ===== WEBDRIVER REMOVAL =====
  // Remove webdriver flag
  Object.defineProperty(navigator, 'webdriver', {
    get: () => false, // Return false instead of undefined (more natural)
    configurable: true
  });
  
  // Remove automation-related properties from navigator prototype
  try {
    delete Object.getPrototypeOf(navigator).webdriver;
  } catch {}
  
  // ===== CHROME OBJECT FIX =====
  if (!window.chrome) {
    window.chrome = {};
  }
  
  if (!window.chrome.runtime) {
    window.chrome.runtime = {
      connect: function() {},
      sendMessage: function() {},
      id: undefined
    };
  }
  
  // Fix chrome.csi for automation detection
  if (!window.chrome.csi) {
    window.chrome.csi = function() {
      return {
        startE: Date.now(),
        onloadT: Date.now() + 100,
        pageT: Date.now() + 150,
        tran: 15
      };
    };
  }
  
  // Fix chrome.loadTimes for automation detection
  if (!window.chrome.loadTimes) {
    window.chrome.loadTimes = function() {
      return {
        commitLoadTime: Date.now() / 1000,
        connectionInfo: "http/1.1",
        finishDocumentLoadTime: Date.now() / 1000 + 0.1,
        finishLoadTime: Date.now() / 1000 + 0.2,
        firstPaintAfterLoadTime: 0,
        firstPaintTime: Date.now() / 1000 + 0.05,
        navigationType: "Other",
        npnNegotiatedProtocol: "unknown",
        requestTime: Date.now() / 1000 - 0.5,
        startLoadTime: Date.now() / 1000 - 0.3,
        wasAlternateProtocolAvailable: false,
        wasFetchedViaSpdy: false,
        wasNpnNegotiated: false
      };
    };
  }
  
  // ===== CDP DETECTION EVASION =====
  // Hide Error.stack traces that reveal puppeteer/CDP
  const originalError = Error;
  window.Error = function(...args) {
    const error = new originalError(...args);
    Object.defineProperty(error, 'stack', {
      get: function() {
        const stack = originalError.prototype.stack;
        if (typeof stack === 'string') {
          // Remove puppeteer/CDP related traces
          return stack
            .split('\n')
            .filter(line => 
              !line.includes('puppeteer') && 
              !line.includes('CDP') &&
              !line.includes('__puppeteer') &&
              !line.includes('devtools')
            )
            .join('\n');
        }
        return stack;
      }
    });
    return error;
  };
  window.Error.prototype = originalError.prototype;
  
  Object.defineProperty(navigator, 'languages', {
    get: () => ['en-US', 'en'],
    configurable: true
  });
  
  // ===== CONNECTION API =====
  Object.defineProperty(navigator, 'connection', {
    get: () => ({
      effectiveType: '4g',
      rtt: 50,
      downlink: 10,
      saveData: false,
      type: 'wifi',
      onchange: null
    }),
    configurable: true
  });
  
  // ===== BATTERY API =====
  navigator.getBattery = () => Promise.resolve({
    charging: true,
    chargingTime: 0,
    dischargingTime: Infinity,
    level: 0.95 + Math.random() * 0.05, // Slight variation
    onchargingchange: null,
    onchargingtimechange: null,
    ondischargingtimechange: null,
    onlevelchange: null
  });
  
  // ===== MEDIA DEVICES =====
  if (navigator.mediaDevices && navigator.mediaDevices.enumerateDevices) {
    const originalEnumerateDevices = navigator.mediaDevices.enumerateDevices.bind(navigator.mediaDevices);
    navigator.mediaDevices.enumerateDevices = async function() {
      const devices = await originalEnumerateDevices();
      // Return at least some devices to appear real
      if (devices.length === 0) {
        return [
          { deviceId: 'default', groupId: 'default', kind: 'audioinput', label: '' },
          { deviceId: 'default', groupId: 'default', kind: 'audiooutput', label: '' },
          { deviceId: 'default', groupId: 'default', kind: 'videoinput', label: '' }
        ];
      }
      return devices;
    };
  }
  
  // ===== IFRAME CONTENTWINDOW FIX =====
  // Some detection checks if iframes have accessible contentWindow
  const originalContentWindow = Object.getOwnPropertyDescriptor(HTMLIFrameElement.prototype, 'contentWindow');
  if (originalContentWindow) {
    Object.defineProperty(HTMLIFrameElement.prototype, 'contentWindow', {
      get: function() {
        const w = originalContentWindow.get.call(this);
        if (w && this.src && this.src.startsWith('about:')) {
          return w;
        }
        return w;
      }
    });
  }
  
  // ===== OUTERWIDTH/HEIGHT FIX =====
  // Bot detection sometimes checks if window dimensions are suspicious
  if (window.outerWidth === 0 || window.outerHeight === 0) {
    Object.defineProperty(window, 'outerWidth', { get: () => window.innerWidth + 16, configurable: true });
    Object.defineProperty(window, 'outerHeight', { get: () => window.innerHeight + 88, configurable: true });
  }
  
  console.log('[browser-profiles] Automation bypass enabled');
})();
