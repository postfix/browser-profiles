(function() {
  const originalQuery = navigator.permissions && navigator.permissions.query ?
    navigator.permissions.query.bind(navigator.permissions) : null;
  const config = %%PERMJSON%%;

  function normalizeDescriptor(parameters) {
    if (typeof parameters === 'string') {
      return parameters;
    }
    if (parameters && typeof parameters === 'object' && parameters.name) {
      return parameters.name;
    }
    return '';
  }

  function createPermissionState(state) {
    return {
      state: state,
      onchange: null,
      addEventListener: function() {},
      removeEventListener: function() {},
      dispatchEvent: function() {}
    };
  }

  navigator.permissions.query = function(parameters) {
    const name = normalizeDescriptor(parameters);
    if (config.hasOwnProperty(name)) {
      return Promise.resolve(createPermissionState(config[name]));
    }
    if (originalQuery) {
      return originalQuery(parameters);
    }
    return Promise.resolve(createPermissionState('prompt'));
  };

  console.log('[browser-profiles] Permissions spoofing enabled');
})();
