
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
      // Per-profile GPU identity via the UNMASKED_* params (what fingerprinters read).
      if (pname === 37445) return %%WEBGL_VENDOR%%; // UNMASKED_VENDOR_WEBGL
      if (pname === 37446) return %%WEBGL_RENDERER%%; // UNMASKED_RENDERER_WEBGL
      if (pname === 7936) return "WebKit"; // VENDOR (masked; generic, matches real Chrome)
      if (pname === 7937) return "WebKit WebGL"; // RENDERER (masked; generic, matches real Chrome)
      if (pname === 7938) return randomItem(["WebGL 1.0", "WebGL 1.0 (OpenGL ES 2.0 Chromium)"]); // VERSION
      if (pname === 35724) return randomItem(["WebGL GLSL ES 1.0", "WebGL GLSL ES 1.0 (OpenGL ES GLSL ES 1.0 Chromium)"]); // SHADING_LANGUAGE_VERSION
      
      // Spoof numeric parameters with deterministic or randomized values
      if (pname === 3379) return %%MAX_TEXTURE_SIZE%%; // MAX_TEXTURE_SIZE
      if (pname === 34076) return %%MAX_CUBE_MAP_TEXTURE_SIZE%%; // MAX_CUBE_MAP_TEXTURE_SIZE
      if (pname === 34024) return %%MAX_RENDERBUFFER_SIZE%%; // MAX_RENDERBUFFER_SIZE
      if (pname === 36347) return %%MAX_VARYING_VECTORS%%; // MAX_VARYING_VECTORS
      if (pname === 36348) return %%MAX_VERTEX_UNIFORM_VECTORS%%; // MAX_VERTEX_UNIFORM_VECTORS
      if (pname === 3386) return %%MAX_VIEWPORT_DIMS%%; // MAX_VIEWPORT_DIMS
      if (pname === 33902) return %%ALIASED_LINE_WIDTH_RANGE%%; // ALIASED_LINE_WIDTH_RANGE
      if (pname === 33901) return %%ALIASED_POINT_SIZE_RANGE%%; // ALIASED_POINT_SIZE_RANGE
      if (pname === 3413) return %%MAX_TEXTURE_IMAGE_UNITS%%; // MAX_TEXTURE_IMAGE_UNITS
      if (pname === 35660) return %%MAX_VERTEX_TEXTURE_IMAGE_UNITS%%; // MAX_VERTEX_TEXTURE_IMAGE_UNITS
      if (pname === 35661) return %%MAX_COMBINED_TEXTURE_IMAGE_UNITS%%; // MAX_COMBINED_TEXTURE_IMAGE_UNITS
      if (pname === 34930) return %%MAX_FRAGMENT_UNIFORM_VECTORS%%; // MAX_FRAGMENT_UNIFORM_VECTORS
      if (pname === 36349) return %%MAX_VERTEX_ATTRIBS%%; // MAX_VERTEX_ATTRIBS
      
      return originalGetParameter.call(this, pname);
    };
  }
  
  // Guarantee the debug-renderer-info extension is present so the spoofed UNMASKED_*
  // params above are the values fingerprinters read. Real headless GPUs (SwiftShader)
  // may return null here, which would leak the generic masked VENDOR/RENDERER instead.
  function spoofGetExtension(proto) {
    const originalGetExtension = proto.getExtension;
    
    proto.getExtension = function(name) {
      if (name === 'WEBGL_debug_renderer_info') {
        return originalGetExtension.call(this, name) || {
          UNMASKED_VENDOR_WEBGL: 37445,
          UNMASKED_RENDERER_WEBGL: 37446
        };
      }
      return originalGetExtension.call(this, name);
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
    spoofGetExtension(WebGLRenderingContext.prototype);
    spoofBufferData(WebGLRenderingContext.prototype);
  }
  
  if (typeof WebGL2RenderingContext !== 'undefined') {
    spoofGetParameter(WebGL2RenderingContext.prototype);
    spoofGetExtension(WebGL2RenderingContext.prototype);
    spoofBufferData(WebGL2RenderingContext.prototype);
  }
  
  console.log('[browser-profiles] WebGL protection enabled');
})();
