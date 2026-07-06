(function() {
  const realCheck = document.fonts && document.fonts.check ?
    document.fonts.check.bind(document.fonts) : null;
  const whitelist = ["Arial","Arial Black","Calibri","Cambria","Cambria Math","Consolas","Courier New","Georgia","Impact","Lucida Console","Lucida Sans Unicode","Microsoft Sans Serif","Segoe UI","Segoe UI Symbol","Tahoma","Times New Roman","Trebuchet MS","Verdana"];
  const whitelistSet = new Set(whitelist.map(function(f) { return f.toLowerCase(); }));

  function parseFamilies(fonts) {
    if (typeof fonts !== 'string') return [];
    return fonts.split(',').map(function(f) {
      const s = f.trim();
      const idx = s.lastIndexOf(' ');
      const name = idx > 0 ? s.slice(idx + 1) : s;
      return name.replace(/^["']|["']$/g, '').trim().toLowerCase();
    }).filter(Boolean);
  }

  document.fonts.check = function(fonts, text) {
    const families = parseFamilies(fonts);
    if (whitelistSet.size === 0 || families.length === 0) {
      return realCheck ? realCheck.apply(document.fonts, arguments) : false;
    }
    for (let i = 0; i < families.length; i++) {
      if (whitelistSet.has(families[i])) {
        return true;
      }
    }
    return realCheck ? realCheck.apply(document.fonts, arguments) : false;
  };

  console.log('[browser-profiles] Fonts guard enabled');
})();
