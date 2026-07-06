(function() {
  const pluginData = %%PLUGINSJSON%%;
  const mimeData = %%MIMETYPESJSON%%;

  function makeMimeType(m) {
    return {
      type: m.type,
      description: m.description,
      suffixes: m.suffixes,
      enabledPlugin: null
    };
  }

  const plugins = pluginData.map(function(p) {
    const mimeTypes = (p.mimeTypes || []).map(makeMimeType);
    const plugin = {
      name: p.name,
      filename: p.filename,
      description: p.description,
      version: p.version,
      length: mimeTypes.length,
      item: function(idx) { return mimeTypes[idx] || null; },
      namedItem: function(name) { return mimeTypes.find(function(x) { return x.type === name; }) || null; }
    };
    mimeTypes.forEach(function(m, i) {
      plugin[i] = m;
      m.enabledPlugin = plugin;
    });
    return plugin;
  });
  plugins.length = pluginData.length;
  plugins.item = function(idx) { return plugins[idx] || null; };
  plugins.namedItem = function(name) { return plugins.find(function(x) { return x.name === name; }) || null; };
  plugins.refresh = function() {};

  const allMimeTypes = mimeData.map(function(m) {
    const mt = makeMimeType(m);
    const plugin = plugins.find(function(p) { return p.name === (m.enabledPlugin || ''); });
    mt.enabledPlugin = plugin || null;
    return mt;
  });
  allMimeTypes.length = mimeData.length;
  allMimeTypes.item = function(idx) { return allMimeTypes[idx] || null; };
  allMimeTypes.namedItem = function(name) { return allMimeTypes.find(function(x) { return x.type === name; }) || null; };

  const pluginArray = new Proxy(plugins, {
    get: function(target, prop) {
      if (prop === 'length') return target.length;
      if (prop === 'item') return target.item;
      if (prop === 'namedItem') return target.namedItem;
      if (prop === 'refresh') return target.refresh;
      if (typeof prop === 'symbol') return target[prop];
      const idx = parseInt(prop, 10);
      if (!isNaN(idx) && idx >= 0 && idx < target.length) return target[idx];
      return target.find(function(x) { return x.name === prop; }) || target[prop];
    }
  });

  const mimeTypeArray = new Proxy(allMimeTypes, {
    get: function(target, prop) {
      if (prop === 'length') return target.length;
      if (prop === 'item') return target.item;
      if (prop === 'namedItem') return target.namedItem;
      if (typeof prop === 'symbol') return target[prop];
      const idx = parseInt(prop, 10);
      if (!isNaN(idx) && idx >= 0 && idx < target.length) return target[idx];
      return target.find(function(x) { return x.type === prop; }) || target[prop];
    }
  });

  Object.defineProperty(navigator, 'plugins', {
    get: function() { return pluginArray; },
    configurable: true
  });
  Object.defineProperty(navigator, 'mimeTypes', {
    get: function() { return mimeTypeArray; },
    configurable: true
  });

  console.log('[browser-profiles] Plugins spoofing enabled');
})();
